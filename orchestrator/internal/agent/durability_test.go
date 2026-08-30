package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/storage"
)

func TestEpochFenceSurvivesRestart(t *testing.T) {
	local := storage.NewLocalStorage(t.TempDir())
	if err := local.Init(); err != nil {
		t.Fatal(err)
	}
	first := &Agent{local: local}
	if err := first.AcceptEpoch(7); err != nil {
		t.Fatal(err)
	}
	second := &Agent{local: local, allocations: map[string]*Allocation{}, orphans: map[string]int{}}
	var epoch uint64
	if err := local.Get("agent/control-epoch", &epoch); err != nil {
		t.Fatal(err)
	}
	second.epoch = epoch
	if err := second.AcceptEpoch(6); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("expected stale epoch, got %v", err)
	}
}

func TestPrepareStartRejectsObsoleteGenerationAndConflict(t *testing.T) {
	agent := &Agent{allocations: map[string]*Allocation{
		"task": {ID: "task", AllocationID: "alloc", Generation: 3, ExecutionHash: "same"},
	}}
	if err := agent.PrepareStart(context.Background(), &api.AllocationRequest{AllocationID: "alloc", Generation: 2}); !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("expected stale generation, got %v", err)
	}
	if err := agent.PrepareStart(context.Background(), &api.AllocationRequest{AllocationID: "alloc", Generation: 3, ExecutionHash: "different"}); !errors.Is(err, ErrExecutionConflict) {
		t.Fatalf("expected metadata conflict, got %v", err)
	}
}

func TestLeaderUnavailableDoesNotConfirmOrphan(t *testing.T) {
	agent := &Agent{allocations: map[string]*Allocation{
		"task": {ID: "task", AllocationID: "alloc", Generation: 1},
	}, orphans: map[string]int{}}
	agent.reconcileDesired(context.Background(), &api.HeartbeatResponse{OrphanConfirmation: false})
	if len(agent.allocations) != 1 || len(agent.orphans) != 0 {
		t.Fatal("an unconfirmed allocation was considered orphaned")
	}
}
