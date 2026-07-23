package runtime

import (
	"context"
)

type ContainerStatus string

const (
	StatusCreated ContainerStatus = "created"
	StatusRunning ContainerStatus = "running"
	StatusStopped ContainerStatus = "stopped"
	StatusUnknown ContainerStatus = "unknown"
)

type CreateOptions struct {
	ID     string
	Image  string
	Env    map[string]string
	Mounts []*Mount
}

type ContainerInfo struct {
	ID     string
	Status ContainerStatus
}

type ContainerRuntime interface {
	Pull(ctx context.Context, image string) error
	Create(ctx context.Context, options CreateOptions) (string, error)
	Start(ctx context.Context, containerId string) error
	Restart(ctx context.Context, containerId string) error
	Stop(ctx context.Context, containerId string) error
	Remove(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, command []string) (int, error)
	Inspect(ctx context.Context, containerID string) (*ContainerInfo, error)
}
