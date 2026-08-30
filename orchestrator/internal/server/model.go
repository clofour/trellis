package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type Cluster struct {
	Hash         string
	ControlEpoch uint64 `json:"control_epoch,omitempty"`
}

type NodeRegistration struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	CPU                int
	Memory             int64
	OS                 string
	Arch               string
	Labels             map[string]string
	WireGuardPublicKey string
	WireGuardEndpoint  string
}

type Node struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	Status             NodeStatus
	LastHeartbeat      time.Time
	CPU                int
	Memory             int64
	OS                 string
	Arch               string
	Labels             map[string]string
	WireGuardPublicKey string
	WireGuardEndpoint  string
}

type NodeStatus string

const (
	NodeStatusHealthy   NodeStatus = "healthy"
	NodeStatusUnhealthy NodeStatus = "unhealthy"
	NodeStatusDraining  NodeStatus = "draining"
)

type NodeSummary struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	CPU                int
	Memory             int64
	OS                 string
	Arch               string
	Labels             map[string]string
	Status             NodeStatus
	WireGuardPublicKey string
	WireGuardEndpoint  string
	LastHeartbeat      time.Time
}

type Job struct {
	Spec     *spec.JobSpec
	Revision int
}

// Allocation is the live control-plane model. Persistence compatibility is
// intentionally handled by allocationRecord at the state boundary so legacy
// fields do not leak into scheduling and reconciliation decisions.
type Allocation struct {
	mu            sync.Mutex
	Namespace     string
	JobName       string
	TaskGroupName string
	ID            string
	Generation    uint64
	JobRevision   int
	Tasks         []spec.TaskSpec
	Phase         lifecycle.Phase
	Health        lifecycle.Health
	lifecycle.Diagnostic
	Node  *Node
	Ports []api.PortMapping

	// Events is an in-memory ring buffer of recent phase transitions. It is not
	// persisted and resets on leader failover.
	Events *lifecycle.RingBuffer
}

func (a *Allocation) AllocationID() string { return a.ID }

func (a *Allocation) normalize(now time.Time) {
	if a.Generation == 0 {
		a.Generation = 1
	}
	if !a.Phase.Valid() {
		a.Phase = lifecycle.PhasePlaced
	}
	if !a.Health.Valid() {
		a.Health = lifecycle.HealthUnknown
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.TransitionedAt.IsZero() {
		a.TransitionedAt = a.CreatedAt
	}
}

func (a *Allocation) compatibilityStatus() string {
	return lifecycle.CompatibilityStatus(a.Phase, a.Health)
}

func (a *Allocation) Transition(to lifecycle.Phase, now time.Time, reason, message string) error {
	a.normalize(now)
	if err := lifecycle.Transition(a.Phase, to); err != nil {
		return err
	}
	if a.Phase != to {
		if a.Events == nil {
			a.Events = &lifecycle.RingBuffer{}
		}
		a.Events.Append(lifecycle.Event{Phase: to, Reason: reason, Message: message, At: now})
		a.Phase = to
		a.TransitionedAt = now
	}
	a.Reason, a.Message = reason, message
	return nil
}

func (a *Allocation) SetHealth(health lifecycle.Health) error {
	if !health.Valid() {
		return fmt.Errorf("invalid allocation health %q", health)
	}
	a.Health = health
	return nil
}
