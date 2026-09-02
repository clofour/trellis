package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/clofour/trellis/internal/state"
)

func TestAcquireLeadershipAdvancesDurableEpoch(t *testing.T) {
	ctx := context.Background()
	store, err := state.NewBoltStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stateCtl := NewStateController(store, "test-cluster")
	if err := stateCtl.PutCluster(ctx, &Cluster{Hash: "hash", ControlEpoch: 3}); err != nil {
		t.Fatal(err)
	}

	// Model a long-lived follower that initialized while epoch 1 was current,
	// then observed two leadership terms without restarting. Leadership must be
	// acquired from the durable epoch, not this stale in-memory snapshot.
	s := &Server{
		state:   stateCtl,
		cluster: &Cluster{Hash: "hash", ControlEpoch: 1},
		now:     time.Now,
	}
	if err := s.AcquireLeadership(ctx); err != nil {
		t.Fatal(err)
	}

	persisted, err := stateCtl.GetCluster(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil || persisted.ControlEpoch != 4 {
		t.Fatalf("durable control epoch = %v, want 4", persisted)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.controlEpoch != 4 {
		t.Fatalf("active control epoch = %d, want 4", s.controlEpoch)
	}
	if s.cluster.ControlEpoch != 4 {
		t.Fatalf("cached cluster control epoch = %d, want 4", s.cluster.ControlEpoch)
	}
}
