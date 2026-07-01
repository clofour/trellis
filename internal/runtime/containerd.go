package runtime

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/models"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const trellisNamespace = "trellis"
const gracePeriod = 10 * time.Second

type ContainerdRuntime struct {
	client *containerd.Client
}

func New(socketPath string) (*ContainerdRuntime, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return nil, err
	}

	return &ContainerdRuntime{
		client: client,
	}, nil
}

func (c *ContainerdRuntime) Close() error {
	return c.client.Close()
}

func (c *ContainerdRuntime) Pull(ctx context.Context, image string) error {
	ctx = c.withNamespace(ctx)

	_, err := c.client.Pull(ctx, image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerdRuntime) Create(ctx context.Context, options CreateOptions) (string, error) {
	ctx = c.withNamespace(ctx)

	image, err := c.client.GetImage(ctx, options.Image)
	if err != nil {
		return "", fmt.Errorf("getting image %s: %w", options.Image, err)
	}

	ociSpecOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithEnv(options.Env),
		oci.WithMounts(convertOci(options.Mounts)),
	}

	container, err := c.client.NewContainer(ctx, options.ID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(options.ID, image),
		containerd.WithNewSpec(ociSpecOpts...),
	)
	if err != nil {
		return "", fmt.Errorf("creating container %s: %w", options.ID, err)
	}

	return container.ID(), nil
}

func (c *ContainerdRuntime) Start(ctx context.Context, containerID string) error {
	ctx = c.withNamespace(ctx)

	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("loading container %s: %w", containerID, err)
	}

	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return fmt.Errorf("creating task for %s: %w", containerID, err)
	}

	err = task.Start(ctx)
	if err != nil {
		task.Delete(ctx)
		return fmt.Errorf("starting task for %s: %w", containerID, err)
	}

	return nil
}

func (c *ContainerdRuntime) Stop(ctx context.Context, containerID string) error {
	ctx = c.withNamespace(ctx)

	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("loading container %s: %w", containerID, err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("getting task for %s: %w", containerID, err)
	}

	exitChannel, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting on task for %s: %w", containerID, err)
	}

	err = task.Kill(ctx, syscall.SIGTERM)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("sending SIGTERM to %s: %w", containerID, err)
	}

	select {
	case <-exitChannel:

	case <-time.After(gracePeriod):
		err := task.Kill(ctx, syscall.SIGKILL)
		if err != nil && !errdefs.IsNotFound(err) {
			return fmt.Errorf("sending SIGKILL to %s: %w", containerID, err)
		}

		select {
		case <-exitChannel:
		case <-time.After(5 * time.Second):
			return fmt.Errorf("container %s did not exit after SIGKILL", containerID)
		}
	}

	_, err = task.Delete(ctx, containerd.WithProcessKill)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("deleting task for %s: %w", containerID, err)
	}

	return nil
}

func (c *ContainerdRuntime) Remove(ctx context.Context, containerID string) error {
	ctx = c.withNamespace(ctx)

	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("loading container %s: %w", containerID, err)
	}

	err = container.Delete(ctx, containerd.WithSnapshotCleanup)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("deleting container %s: %w", containerID, err)
	}

	return nil
}

func (c *ContainerdRuntime) Exec(ctx context.Context, containerID string, command []string) (int, error) {
	ctx = c.withNamespace(ctx)

	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return 1, fmt.Errorf("loading container %s: %w", containerID, err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return 1, fmt.Errorf("getting task for %s: %w", containerID, err)
	}

	execID := fmt.Sprintf("healthcheck-%d", time.Now().UnixNano())
	process := &specs.Process{
		Args: command,
		Cwd:  "/",
	}

	taskExec, err := task.Exec(ctx, execID, process, cio.NullIO)
	if err != nil {
		return 1, fmt.Errorf("constructing command %s: %w", command, err)
	}
	defer taskExec.Delete(ctx)

	exitChannel, err := taskExec.Wait(ctx)
	if err != nil {
		return 1, fmt.Errorf("waiting on command %s: %w", command, err)
	}

	err = taskExec.Start(ctx)
	if err != nil {
		return 1, fmt.Errorf("executing command %s: %w", command, err)
	}

	status := <-exitChannel
	code, _, err := status.Result()
	if err != nil {
		return 1, fmt.Errorf("extracting status %s: %w", command, err)
	}

	return int(code), nil
}

func (c *ContainerdRuntime) Inspect(ctx context.Context, containerID string) (*ContainerInfo, error) {
	ctx = c.withNamespace(ctx)

	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("loading container %s: %w", containerID, err)
	}

	info, err := container.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting info for %s: %w", containerID, err)
	}

	result := &ContainerInfo{
		ID:     info.ID,
		Status: StatusUnknown,
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			result.Status = StatusStopped
			return result, nil
		}

		return nil, fmt.Errorf("getting task for %s: %w", containerID, err)
	}

	rawStatus, err := task.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting task status for %s: %w", containerID, err)
	}

	switch rawStatus.Status {
	case containerd.Created:
		result.Status = StatusCreated
	case containerd.Running:
		result.Status = StatusRunning
	case containerd.Stopped:
		result.Status = StatusStopped
	default:
		result.Status = StatusUnknown
	}

	return result, nil
}

func convertOci(mounts []models.MountSpec) []specs.Mount {
	result := make([]specs.Mount, len(mounts))

	for i, m := range mounts {

		result[i] = specs.Mount{
			Source:      m.HostPath,
			Destination: m.ContainerPath,
			Type:        "bind",
			Options:     []string{"rbind", "rw"},
		}

	}

	return result
}

func (c *ContainerdRuntime) withNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, trellisNamespace)
}
