package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

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
	logDir string
}

type Port struct {
	HostPort      int
	ContainerPort int
}

type Mount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

func NewContainerdRuntime(socketPath string) (*ContainerdRuntime, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return nil, err
	}

	return &ContainerdRuntime{
		client: client,
		logDir: filepath.Join(os.TempDir(), "trellis-logs"),
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

	allMounts := convertMounts(options.Mounts)
	if len(options.DNSServers) > 0 {
		resolvPath := filepath.Join(c.logDir, options.ID+"-resolv.conf")
		if err := writeDNSConfig(resolvPath, options.DNSServers); err != nil {
			return "", fmt.Errorf("write resolv.conf for %s: %w", options.ID, err)
		}
		allMounts = append(allMounts, specs.Mount{
			Source:      resolvPath,
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Options:     []string{"rbind", "ro"},
		})
	}
	ociSpecOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithEnv(convertEnv(options.Env)),
		oci.WithMounts(allMounts),
	}
	if options.NetworkNamespace != "" {
		ociSpecOpts = append(ociSpecOpts, oci.WithLinuxNamespace(specs.LinuxNamespace{
			Type: specs.NetworkNamespace, Path: options.NetworkNamespace,
		}))
	}
	if options.CPU > 0 {
		ociSpecOpts = append(ociSpecOpts, oci.WithCPUCFS(int64(options.CPU*100), 100000))
	}
	if options.Memory > 0 {
		ociSpecOpts = append(ociSpecOpts, oci.WithMemoryLimit(uint64(options.Memory)))
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithNewSnapshot(options.ID, image),
		containerd.WithNewSpec(ociSpecOpts...),
	}
	if len(options.Labels) > 0 {
		containerOpts = append(containerOpts, containerd.WithContainerLabels(options.Labels))
	}
	if options.Runtime != "" {
		if options.Runtime != "runsc" {
			return "", fmt.Errorf("unsupported runtime %q", options.Runtime)
		}
		containerOpts = append(containerOpts, containerd.WithRuntime("io.containerd.runsc.v1", nil))
	}
	container, err := c.client.NewContainer(ctx, options.ID, containerOpts...)
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

	if err := os.MkdirAll(c.logDir, 0o750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	task, err := container.NewTask(ctx, cio.LogFile(c.logPath(containerID)))
	if err != nil {
		return fmt.Errorf("creating task for %s: %w", containerID, err)
	}

	err = task.Start(ctx)
	if err != nil {
		_, _ = task.Delete(ctx)
		return fmt.Errorf("starting task for %s: %w", containerID, err)
	}

	return nil
}

func (c *ContainerdRuntime) logPath(containerID string) string {
	return filepath.Join(c.logDir, filepath.Base(containerID)+".log")
}

func (c *ContainerdRuntime) Logs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
	file, err := os.Open(c.logPath(containerID))
	if err != nil {
		return nil, fmt.Errorf("open logs for %s: %w", containerID, err)
	}
	return newLogReader(ctx, file, follow, tail)
}

func (c *ContainerdRuntime) Restart(ctx context.Context, containerID string) error {
	err := c.Stop(ctx, containerID)
	if err != nil {
		return fmt.Errorf("stopping container %s: %w", containerID, err)
	}

	err = c.Start(ctx, containerID)
	if err != nil {
		return fmt.Errorf("starting container %s: %w", containerID, err)
	}

	return nil
}

func (c *ContainerdRuntime) Stop(ctx context.Context, containerID string) error {
	ctx = c.withNamespace(ctx)

	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
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
		if errdefs.IsNotFound(err) {
			return nil
		}
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
		Labels: info.Labels,
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

func (c *ContainerdRuntime) ListManaged(ctx context.Context, cluster string) ([]ContainerInfo, error) {
	ctx = c.withNamespace(ctx)
	containers, err := c.client.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	result := make([]ContainerInfo, 0, len(containers))
	for _, container := range containers {
		info, err := container.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("inspect container %s: %w", container.ID(), err)
		}
		if info.Labels["trellis.cluster"] != cluster {
			continue
		}
		observed, err := c.Inspect(ctx, container.ID())
		if err != nil {
			return nil, err
		}
		observed.Labels = info.Labels
		result = append(result, *observed)
	}
	return result, nil
}

func convertEnv(envMap map[string]string) []string {
	env := make([]string, 0, len(envMap))

	for k, v := range envMap {
		env = append(env, k+"="+v)
	}

	return env
}

func convertMounts(mounts []*Mount) []specs.Mount {
	result := make([]specs.Mount, len(mounts))

	for i, m := range mounts {

		mode := "rw"
		if m.ReadOnly {
			mode = "ro"
		}
		result[i] = specs.Mount{
			Source:      m.HostPath,
			Destination: m.ContainerPath,
			Type:        "bind",
			Options:     []string{"rbind", mode},
		}

	}

	return result
}

func writeDNSConfig(path string, servers []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create DNS config directory: %w", err)
	}
	var content string
	for _, s := range servers {
		content += "nameserver " + s + "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (c *ContainerdRuntime) withNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, trellisNamespace)
}
