package election

import (
	"context"

	"github.com/hashicorp/raft"
)

var _ Elector = (*RaftElector)(nil)

type RaftElector struct {
	raft *raft.Raft
	self Leader
}

func NewRaftElector(r *raft.Raft, self Leader) *RaftElector {
	return &RaftElector{raft: r, self: self}
}

func (e *RaftElector) Run(ctx context.Context, events chan<- Event) error {
	ch := e.raft.LeaderCh()
	for {
		select {
		case isLeader := <-ch:
			events <- Event{Leader: e.self, Elected: isLeader}
		case <-ctx.Done():
			return nil
		}
	}
}

func (e *RaftElector) Current(_ context.Context) (*Leader, error) {
	_, id := e.raft.LeaderWithID()
	if id == "" {
		return nil, nil
	}
	return &Leader{Address: string(id)}, nil
}
