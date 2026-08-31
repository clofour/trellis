package lifecycle

import (
	"fmt"
	"time"
)

// Phase describes allocation execution. Health is intentionally orthogonal.
type Phase string

const (
	// PhasePlaced describes an allocation accepted for placement.
	PhasePlaced Phase = "placed"
	// PhaseStarting and the following values describe subsequent execution states.
	PhaseStarting Phase = "starting"
	// PhaseRunning indicates that the allocation is running.
	PhaseRunning Phase = "running"
	// PhaseStopping indicates that the allocation is stopping.
	PhaseStopping Phase = "stopping"
	// PhaseStopped indicates that the allocation has stopped.
	PhaseStopped Phase = "stopped"
	// PhaseFailed indicates that the allocation failed.
	PhaseFailed Phase = "failed"
	// PhaseLost indicates that the allocation was lost.
	PhaseLost Phase = "lost"
)

// Health describes the health-check state of an allocation.
type Health string

const (
	// HealthUnknown indicates that health is not yet known.
	HealthUnknown Health = "unknown"
	// HealthHealthy and HealthUnhealthy describe completed health checks.
	HealthHealthy Health = "healthy"
	// HealthUnhealthy indicates that a health check failed.
	HealthUnhealthy Health = "unhealthy"
)

var transitions = map[Phase]map[Phase]bool{
	PhasePlaced:   {PhaseStarting: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
	PhaseStarting: {PhaseRunning: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
	PhaseRunning:  {PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
	PhaseStopping: {PhaseStopped: true, PhaseFailed: true, PhaseLost: true},
	PhaseStopped:  {PhaseStarting: true},
	PhaseFailed:   {PhaseStarting: true, PhaseStopping: true, PhaseLost: true},
	PhaseLost:     {PhaseStarting: true, PhaseStopping: true, PhaseStopped: true},
}

// Valid reports whether p is a recognized phase.
func (p Phase) Valid() bool {
	_, ok := transitions[p]
	return ok
}

// Valid reports whether h is a recognized health state.
func (h Health) Valid() bool {
	return h == HealthUnknown || h == HealthHealthy || h == HealthUnhealthy
}

// CanTransition reports whether an allocation can move between two phases.
func CanTransition(from, to Phase) bool {
	return from == to || transitions[from][to]
}

// Transition validates an allocation phase change.
func Transition(from, to Phase) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("unknown allocation phase transition %q -> %q", from, to)
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid allocation phase transition %q -> %q", from, to)
	}
	return nil
}

// Legacy converts the pre-lifecycle status values without claiming more than
// the old record actually proves. A healthy or unhealthy allocation had been
// started; pending only proved placement.
func Legacy(status string) (Phase, Health) {
	switch status {
	case "healthy":
		return PhaseRunning, HealthHealthy
	case "unhealthy":
		return PhaseRunning, HealthUnhealthy
	case "running":
		return PhaseRunning, HealthUnknown
	case "stopped":
		return PhaseStopped, HealthUnknown
	case "failed":
		return PhaseFailed, HealthUnknown
	case "lost":
		return PhaseLost, HealthUnknown
	default:
		return PhasePlaced, HealthUnknown
	}
}

// CompatibilityStatus preserves the old status field for existing clients.
func CompatibilityStatus(phase Phase, health Health) string {
	if phase == PhaseRunning {
		if health == HealthHealthy {
			return "healthy"
		}
		if health == HealthUnhealthy {
			return "unhealthy"
		}
	}
	return string(phase)
}

// Diagnostic records details about the latest allocation transition.
type Diagnostic struct {
	CreatedAt      time.Time  `json:"created_at"`
	TransitionedAt time.Time  `json:"last_transition_at"`
	Reason         string     `json:"reason,omitempty"`
	Message        string     `json:"message,omitempty"`
	Attempt        int        `json:"attempt"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
}
