package election

import (
	"context"

	"github.com/google/uuid"
)

type Leader struct {
	NodeID  uuid.UUID `json:"node_id"`
	Address string    `json:"address"`
}

type Event struct {
	Leader  Leader
	Elected bool
}

type Elector interface {
	Run(ctx context.Context, events chan<- Event) error
	Current(ctx context.Context) (*Leader, error)
}

type SingleNodeElector struct {
	leader Leader
}

func NewSingleNodeElector(leader Leader) *SingleNodeElector {
	return &SingleNodeElector{leader: leader}
}

func (e *SingleNodeElector) Run(ctx context.Context, events chan<- Event) error {
	select {
	case events <- Event{Leader: e.leader, Elected: true}:
	case <-ctx.Done():
		return nil
	}
	<-ctx.Done()
	return nil
}

func (e *SingleNodeElector) Current(_ context.Context) (*Leader, error) {
	return &e.leader, nil
}
