package runtime

import (
	"context"
	"io"
)

// ContainerStatus describes the observed state of a container.
type ContainerStatus string

const (
	// StatusCreated indicates that a container has been created.
	StatusCreated ContainerStatus = "created"
	// StatusRunning indicates that a container is running.
	StatusRunning ContainerStatus = "running"
	// StatusStopped indicates that a container has stopped.
	StatusStopped ContainerStatus = "stopped"
	// StatusUnknown indicates that container state is unavailable.
	StatusUnknown ContainerStatus = "unknown"
)

// CreateOptions configures a new allocation container.
type CreateOptions struct {
	ID               string
	Image            string
	Env              map[string]string
	Mounts           []*Mount
	CPU              int
	Memory           int64
	Runtime          string
	NetworkNamespace string
	DNSServers       []string
	Labels           map[string]string
}

// ContainerInfo describes a managed container.
type ContainerInfo struct {
	ID     string
	Status ContainerStatus
	Labels map[string]string
}

// ManagedRuntime is implemented by runtimes that can inventory Trellis-owned
// containers for restart adoption and confirmed orphan collection.
type ManagedRuntime interface {
	ListManaged(ctx context.Context, cluster string) ([]ContainerInfo, error)
}

// ContainerRuntime defines the operations required by an allocation runtime.
type ContainerRuntime interface {
	Pull(ctx context.Context, image string) error
	Create(ctx context.Context, options CreateOptions) (string, error)
	Start(ctx context.Context, containerID string) error
	Restart(ctx context.Context, containerID string) error
	Stop(ctx context.Context, containerID string) error
	Remove(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, command []string) (int, error)
	Inspect(ctx context.Context, containerID string) (*ContainerInfo, error)
	Logs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error)
}
