package election

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type Leader struct {
	NodeID  uuid.UUID `json:"node_id"`
	Address string    `json:"address"`
}

type Event struct {
	Leader  Leader
	Elected bool
}

type Elector struct {
	client *api.Client
	key    string
	leader Leader
	ttl    time.Duration
}

func New(client *api.Client, cluster string, leader Leader, ttl time.Duration) *Elector {
	return &Elector{client: client, key: fmt.Sprintf("trellis/%s/leader", cluster), leader: leader, ttl: ttl}
}

func (e *Elector) Run(ctx context.Context, events chan<- Event) error {
	value, err := json.Marshal(e.leader)
	if err != nil {
		return fmt.Errorf("marshal leader: %w", err)
	}
	for ctx.Err() == nil {
		lock, err := e.client.LockOpts(&api.LockOptions{Key: e.key, Value: value, SessionTTL: e.ttl.String(), LockWaitTime: time.Second, LockDelay: time.Second, MonitorRetries: 2})
		if err != nil {
			return fmt.Errorf("create leader lock: %w", err)
		}
		lost, err := lock.Lock(ctx.Done())
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				continue
			}
		}
		if lost == nil {
			return nil
		}
		select {
		case events <- Event{Leader: e.leader, Elected: true}:
		case <-ctx.Done():
			_ = lock.Unlock()
			return nil
		}
		select {
		case <-lost:
		case <-ctx.Done():
			_ = lock.Unlock()
			return nil
		}
		select {
		case events <- Event{Leader: e.leader, Elected: false}:
		case <-ctx.Done():
			return nil
		}
		_ = lock.Unlock()
	}
	return nil
}

func (e *Elector) Current(ctx context.Context) (*Leader, error) {
	pair, _, err := e.client.KV().Get(e.key, (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("read leader: %w", err)
	}
	if pair == nil || pair.Session == "" {
		return nil, nil
	}
	var leader Leader
	if err := json.Unmarshal(pair.Value, &leader); err != nil {
		return nil, fmt.Errorf("decode leader: %w", err)
	}
	return &leader, nil
}
