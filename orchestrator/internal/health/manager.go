package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
)

const (
	defaultCheckInterval  = 10 * time.Second
	defaultCheckTimeout   = 5 * time.Second
	defaultCheckThreshold = 3
)

type HealthSubscriber interface {
	OnHealthy(ctx context.Context, allocID string) error
	OnUnhealthy(ctx context.Context, allocID string) error
}

type HealthConfig struct {
	Type      spec.HealthCheckType
	Addr      string
	Port      int
	Path      string
	Command   []string
	Interval  time.Duration
	Timeout   time.Duration
	Threshold int
}

type trackedTask struct {
	allocID     string
	containerID string
	config      HealthConfig
	health      *TaskHealth
	cancel      context.CancelFunc
}

type HealthManager struct {
	log        *slog.Logger
	runtime    runtime.ContainerRuntime
	Subscriber HealthSubscriber
	mu         sync.Mutex
	tasks      map[string]*trackedTask
	ctx        context.Context
}

func NewHealthManager(log *slog.Logger, runtime runtime.ContainerRuntime, subscriber HealthSubscriber) *HealthManager {
	return &HealthManager{log: log, runtime: runtime, tasks: make(map[string]*trackedTask), Subscriber: subscriber, ctx: context.Background()}
}

func (h *HealthManager) SetContext(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ctx = ctx
}

func (h *HealthManager) RegisterTask(allocID string, containerID string, check *spec.HealthCheckSpec) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx, cancel := context.WithCancel(h.ctx)
	if existing := h.tasks[allocID]; existing != nil {
		existing.cancel()
		delete(h.tasks, allocID)
	}
	config := newHealthConfig(check)
	h.tasks[allocID] = &trackedTask{allocID: allocID, containerID: containerID, config: config, health: NewTaskHealth(config.Threshold), cancel: cancel}
	go h.runHealthCheckLoop(ctx, h.tasks[allocID])
}

func newHealthConfig(check *spec.HealthCheckSpec) HealthConfig {
	config := HealthConfig{Type: check.Type, Addr: "127.0.0.1", Port: check.Port, Path: check.Path, Command: check.Command, Interval: defaultCheckInterval, Timeout: defaultCheckTimeout, Threshold: defaultCheckThreshold}
	if check.Interval != 0 {
		config.Interval = check.Interval
	}
	if check.Timeout != 0 {
		config.Timeout = check.Timeout
	}
	if check.Threshold != 0 {
		config.Threshold = check.Threshold
	}
	return config
}

func (h *HealthManager) DeregisterTask(allocID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if tracked := h.tasks[allocID]; tracked != nil {
		tracked.cancel()
		delete(h.tasks, allocID)
	}
}

func (h *HealthManager) runHealthCheckLoop(ctx context.Context, tracked *trackedTask) {
	ticker := time.NewTicker(tracked.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := h.runHealthCheck(ctx, tracked)
			if err != nil {
				h.log.Error("health check failed", "error", err)
				result = false
			}
			h.mu.Lock()
			change, status := tracked.health.RecordResult(result)
			h.mu.Unlock()
			if !change {
				continue
			}
			var callbackErr error
			switch status {
			case StatusHealthy:
				callbackErr = h.Subscriber.OnHealthy(ctx, tracked.allocID)
			case StatusUnhealthy:
				callbackErr = h.Subscriber.OnUnhealthy(ctx, tracked.allocID)
			}
			if callbackErr != nil {
				h.log.Error("health status callback failed", "status", status, "error", callbackErr)
			}
		}
	}
}

func (h *HealthManager) runHealthCheck(ctx context.Context, tracked *trackedTask) (bool, error) {
	config := tracked.config
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	switch config.Type {
	case spec.HealthCheckHTTP:
		return CheckHTTP(ctx, config.Addr, config.Port, config.Path)
	case spec.HealthCheckTCP:
		return CheckTCP(ctx, config.Addr, config.Port)
	case spec.HealthCheckScript:
		return CheckScript(ctx, h.runtime, tracked.containerID, config.Command)
	default:
		return false, fmt.Errorf("unknown check type %s", config.Type)
	}
}
