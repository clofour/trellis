package agent

import (
	"context"
	"fmt"

	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/runtime"
)

type Agent struct {
	runtime runtime.ContainerRuntime
	health  *health.HealthManager
	// discovery registr
	// ports *agent.PortManager
	volumes *VolumeManager
}

type Allocation struct {
	JobName   string
	GroupName string
	TaskName  string
	Spec      models.TaskSpec

	ContainerID string
	ServiceID   string
	Ports       []models.Port
	Mounts      []models.Mount
}

func NewAgent(runtime runtime.ContainerRuntime, health *health.HealthManager, volumes *VolumeManager) *Agent {
	return &Agent{
		runtime: runtime,
		health:  health,
		volumes: volumes,
	}
}

func (a *Agent) RunAllocation(ctx context.Context, jobName string, groupName string, taskName string, spec *models.TaskSpec) error {
	// Validate spec
	allocID := fmt.Sprintf("%s-%s-%s-%d", jobName, groupName, taskName)

	var mounts []*models.Mount
	for _, v := range spec.Volumes {
		mount, err := a.volumes.Create(jobName, taskName, v)
		if err != nil {
			return fmt.Errorf("create volume %s: %w", v.Name, err)
		}

		mounts = append(mounts, mount)
	}

	err := a.runtime.Pull(ctx, spec.Image)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", spec.Image, err)
	}

	containerID := allocID
	_, err = a.runtime.Create(ctx, runtime.CreateOptions{
		ID:     containerID,
		Image:  spec.Image,
		Env:    spec.Env,
		Mounts: mounts,
	})
	if err != nil {
		return fmt.Errorf("create container %s: %w", containerID, err)
	}

	err = a.runtime.Start(ctx, containerID)
	if err != nil {
		return fmt.Errorf("start container %s: %w", containerID, err)
	}

	alloc := &Allocation{}

	return nil
}

func (a *Agent) StopAllocation(ctx context.Context)
