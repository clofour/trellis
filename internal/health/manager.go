package health

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
)

const checkInterval = 10 * time.Second
const checkTimeout = 5 * time.Second
const checkThreshold = 3

type HealthSubscriber interface {
	OnHealthy(ctx context.Context, allocID string) error
	OnUnhealthy(ctx context.Context, allocID string) error
}

type HealthConfig struct {
	Type    string
	Addr    string
	Port    int
	Path    string
	Command []string
}

type trackedTask struct {
	allocID     string
	containerID string

	config HealthConfig
	health *TaskHealth
	cancel context.CancelFunc
}

type HealthManager struct {
	mtx        sync.Mutex
	runtime    runtime.ContainerRuntime
	tasks      map[string]*trackedTask
	Subscriber HealthSubscriber
}

func NewHealthManager(runtime runtime.ContainerRuntime, subscriber HealthSubscriber) *HealthManager {
	return &HealthManager{
		runtime:    runtime,
		tasks:      make(map[string]*trackedTask),
		Subscriber: subscriber,
	}
}

func (h *HealthManager) RegisterTask(allocID string, containerID string, spec *spec.HealthCheckSpec) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	existingTrackedTask, ok := h.tasks[allocID]
	if ok {
		existingTrackedTask.cancel()
		delete(h.tasks, allocID)
	}

	config := HealthConfig{
		Type:    spec.Type,
		Addr:    "127.0.0.1",
		Port:    spec.Port,
		Path:    spec.Path,
		Command: spec.Command,
	}

	newTrackedTask := &trackedTask{
		allocID:     allocID,
		containerID: containerID,
		config:      config,
		health:      NewTaskHealth(),
		cancel:      cancel,
	}
	h.tasks[allocID] = newTrackedTask

	go h.runHealthCheckLoop(ctx, newTrackedTask)
}

func (h *HealthManager) DeregisterTask(allocID string) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	trackedTask, ok := h.tasks[allocID]
	if ok {
		trackedTask.cancel()
		delete(h.tasks, allocID)
	}
}

func (h *HealthManager) runHealthCheckLoop(ctx context.Context, trackedTask *trackedTask) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := h.runHealthCheck(ctx, trackedTask)
			if err != nil {
				log.Print(fmt.Errorf("health check: %w", err))
				result = false
			}

			h.mtx.Lock()
			change, status := trackedTask.health.RecordResult(result)
			h.mtx.Unlock()

			if change {
				switch status {
				case StatusHealthy:
					h.Subscriber.OnHealthy(ctx, trackedTask.allocID)
				case StatusUnhealthy:
					h.Subscriber.OnUnhealthy(ctx, trackedTask.allocID)
				}
			}
		}
	}
}

func (h *HealthManager) runHealthCheck(ctx context.Context, trackedTask *trackedTask) (bool, error) {
	config := trackedTask.config

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	switch config.Type {
	case "http":
		return CheckHTTP(ctx, config.Addr, config.Port, config.Path)
	case "tcp":
		return CheckTCP(ctx, config.Addr, config.Port)
	case "script":
		return CheckScript(ctx, h.runtime, trackedTask.containerID, config.Command)
	default:
		return false, fmt.Errorf("unknown check type %s", config.Type)
	}
}
