package health

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/runtime"
)

const checkInterval = 10 * time.Second
const checkTimeout = 5 * time.Second
const checkThreshold = 3

type HealthSubscriber interface {
	OnHealthy(taskID string)
	OnUnhealthy(taskID string)
}

type HealthConfig struct {
	ContainerID string
	TaskID      string

	Type    string
	Addr    string
	Port    int
	Path    string
	Command []string
}

type trackedTask struct {
	config HealthConfig
	health *TaskHealth
	cancel context.CancelFunc
}

type HealthManager struct {
	mtx      sync.Mutex
	runtime  runtime.ContainerRuntime
	tasks    map[string]*trackedTask
	consumer HealthSubscriber
}

func NewHealthManager(runtime runtime.ContainerRuntime, consumer HealthSubscriber) *HealthManager {
	return &HealthManager{
		runtime:  runtime,
		tasks:    make(map[string]*trackedTask),
		consumer: consumer,
	}
}

func (h *HealthManager) RegisterTask(config HealthConfig) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	existingTrackedTask, ok := h.tasks[config.TaskID]
	if ok {
		existingTrackedTask.cancel()
		delete(h.tasks, config.TaskID)
	}

	newTrackedTask := &trackedTask{
		config: config,
		health: NewTaskHealth(),
		cancel: cancel,
	}
	h.tasks[config.TaskID] = newTrackedTask

	go h.runHealthCheckLoop(ctx, newTrackedTask)
}

func (h *HealthManager) DeregisterTask(taskID string) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	trackedTask, ok := h.tasks[taskID]
	if ok {
		trackedTask.cancel()
		delete(h.tasks, taskID)
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
					h.consumer.OnHealthy(trackedTask.config.TaskID)
				case StatusUnhealthy:
					h.consumer.OnUnhealthy(trackedTask.config.TaskID)
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
		return CheckScript(ctx, h.runtime, config.ContainerID, config.Command)
	default:
		return false, fmt.Errorf("unknown check type %s", config.Type)
	}
}
