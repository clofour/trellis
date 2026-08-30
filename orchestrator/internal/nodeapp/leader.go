package nodeapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/election"
	"github.com/clofour/trellis/internal/server"
	"github.com/google/uuid"
)

const shutdownTime = 10 * time.Second

func runLeaderLoop(ctx context.Context, log *slog.Logger, cfg *Config, id uuid.UUID, control *server.Server, events <-chan election.Event, serveLeader func(context.Context) error) error {
	var leaderCancel context.CancelFunc
	var leaderDone <-chan struct{}

	for {
		select {
		case <-ctx.Done():
			if leaderCancel != nil {
				leaderCancel()
			}
			return nil
		case event, ok := <-events:
			if !ok {
				return fmt.Errorf("leader election event stream closed")
			}
			if event.Elected {
				if err := control.Reload(ctx); err != nil {
					return fmt.Errorf("load leader state: %w", err)
				}
				if err := control.AcquireLeadership(ctx); err != nil {
					return fmt.Errorf("advance leadership epoch: %w", err)
				}

				termCtx, cancel := context.WithCancel(ctx)
				leaderCancel = cancel
				log.Info("leadership acquired", "node_id", id, "address", cfg.ServerAdvertise)
				control.Run(termCtx)

				done := make(chan struct{})
				leaderDone = done
				go func() {
					defer close(done)
					if err := serveLeader(termCtx); err != nil && termCtx.Err() == nil {
						log.Error("leader API stopped", "error", err)
						cancel()
					}
				}()
			} else if leaderCancel != nil {
				log.Warn("leadership lost", "node_id", id)
				leaderCancel()
				if leaderDone != nil {
					select {
					case <-leaderDone:
					case <-time.After(shutdownTime):
						log.Error("leader API shutdown timed out")
					}
				}
				leaderCancel = nil
				leaderDone = nil
			}
		}
	}
}

func watchLeader(ctx context.Context, log *slog.Logger, elector election.Elector, target *client.ServerClient) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		leader, err := elector.Current(ctx)
		if err != nil {
			log.Error("discover leader failed", "error", err)
		} else if leader != nil {
			target.SetAddress(leader.Address)
		} else {
			target.SetAddress("")
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
