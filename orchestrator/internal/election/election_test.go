package election

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSingleNodeElectorEmitsElectedEvent(t *testing.T) {
	id := uuid.New()
	elector := NewSingleNodeElector(Leader{NodeID: id, Address: "node:8128"})
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 1)
	done := make(chan error, 1)
	go func() { done <- elector.Run(ctx, events) }()

	select {
	case e := <-events:
		if !e.Elected {
			t.Fatal("expected elected event")
		}
		if e.Leader.NodeID != id || e.Leader.Address != "node:8128" {
			t.Fatalf("unexpected leader: %#v", e.Leader)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for election event")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Run to exit")
	}
}

func TestSingleNodeElectorCurrentReturnsLeader(t *testing.T) {
	id := uuid.New()
	elector := NewSingleNodeElector(Leader{NodeID: id, Address: "node:8128"})

	leader, err := elector.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if leader == nil || leader.NodeID != id || leader.Address != "node:8128" {
		t.Fatalf("unexpected leader: %#v", leader)
	}
}

var _ Elector = (*SingleNodeElector)(nil)
