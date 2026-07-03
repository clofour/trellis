package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/runtime"
)

const checkInterval = 3 * time.Second
const maxRestarts = 3
const windowSize = 10 * time.Minute

type RestartController struct {
	runtime runtime.ContainerRuntime

	mtx    sync.Mutex
	states map[string]*restartState

	subscriber RestartSubscriber
}

type RestartSubscriber interface {
	OnFailed(allocID string)
}

type restartState struct {
	restarting bool
	attempts   int
	window     time.Time
}

func NewRestartController(runtime runtime.ContainerRuntime, subscriber RestartSubscriber) *RestartController {
	return &RestartController{
		runtime: runtime,

		states: make(map[string]*restartState),

		subscriber: subscriber,
	}
}

func (r *RestartController) Track(ctx context.Context, allocID string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.states[allocID] = &restartState{
		restarting: false,
		attempts:   0,
		window:     time.Now(),
	}
}

func (r *RestartController) Untrack(ctx context.Context, allocID string) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	_, ok := r.states[allocID]
	if !ok {
		return fmt.Errorf("alloc %s not tracked", allocID)
	}

	delete(r.states, allocID)

	return nil
}

func (r *RestartController) RunDetectionLoop(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			r.mtx.Lock()
			allocIDs := make([]string, 0, len(r.states))
			for allocID := range r.states {
				allocIDs = append(allocIDs, allocID)
			}
			r.mtx.Unlock()

			for _, allocID := range allocIDs {
				containerState, err := r.runtime.Inspect(ctx, allocID)
				if err != nil {
					// error?
				}

				if containerState.Status == runtime.StatusStopped {
					r.RequestRestart(ctx, allocID)
				}
			}

		}
	}
}

func (r *RestartController) RequestRestart(ctx context.Context, allocID string) error {
	r.mtx.Lock()
	state, ok := r.states[allocID]
	if !ok {
		r.mtx.Unlock()
		return fmt.Errorf("alloc %s not tracked", allocID)
	}

	if state.restarting == true {
		r.mtx.Unlock()
		return nil
	}
	state.restarting = true

	now := time.Now()
	if now.Sub(state.window) > windowSize {
		state.attempts = 0
		state.window = now
	}

	if state.attempts >= maxRestarts {
		state.restarting = false
		r.mtx.Unlock()

		r.subscriber.OnFailed(allocID)
		return nil
	}

	state.attempts++

	r.mtx.Unlock()
	err := r.runtime.Restart(ctx, allocID)
	if err != nil {
		r.mtx.Lock()
		state.restarting = false
		r.mtx.Unlock()

		return fmt.Errorf("restart alloc %s: %w", allocID, err)
	}

	r.mtx.Lock()
	state.restarting = false
	r.mtx.Unlock()

	return nil
}
