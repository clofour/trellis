package lifecycle

import "testing"

func TestTransitionMatrix(t *testing.T) {
	phases := []Phase{PhasePlaced, PhaseStarting, PhaseRunning, PhaseStopping, PhaseStopped, PhaseFailed, PhaseLost}
	allowed := map[Phase]map[Phase]bool{
		PhasePlaced:   {PhasePlaced: true, PhaseStarting: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
		PhaseStarting: {PhaseStarting: true, PhaseRunning: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
		PhaseRunning:  {PhaseRunning: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
		PhaseStopping: {PhaseStopping: true, PhaseStopped: true, PhaseFailed: true, PhaseLost: true},
		PhaseStopped:  {PhaseStopped: true, PhaseStarting: true},
		PhaseFailed:   {PhaseFailed: true, PhaseStarting: true, PhaseStopping: true, PhaseLost: true},
		PhaseLost:     {PhaseLost: true, PhaseStarting: true, PhaseStopping: true, PhaseStopped: true},
	}

	for _, from := range phases {
		if !from.Valid() {
			t.Fatalf("known phase %q is invalid", from)
		}
		for _, to := range phases {
			want := allowed[from][to]
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%q, %q) = %v, want %v", from, to, got, want)
			}
			err := Transition(from, to)
			if want && err != nil {
				t.Errorf("Transition(%q, %q) unexpectedly failed: %v", from, to, err)
			}
			if !want && err == nil {
				t.Errorf("Transition(%q, %q) unexpectedly succeeded", from, to)
			}
		}
	}
	if err := Transition(Phase("bogus"), PhaseRunning); err == nil {
		t.Fatal("unknown source phase must be rejected")
	}
	if err := Transition(PhaseRunning, Phase("bogus")); err == nil {
		t.Fatal("unknown destination phase must be rejected")
	}
}

func TestLegacyStatusMapping(t *testing.T) {
	tests := map[string]struct {
		phase  Phase
		health Health
	}{
		"pending":   {PhasePlaced, HealthUnknown},
		"healthy":   {PhaseRunning, HealthHealthy},
		"unhealthy": {PhaseRunning, HealthUnhealthy},
		"running":   {PhaseRunning, HealthUnknown},
		"stopped":   {PhaseStopped, HealthUnknown},
		"failed":    {PhaseFailed, HealthUnknown},
		"lost":      {PhaseLost, HealthUnknown},
		"unknown":   {PhasePlaced, HealthUnknown},
	}
	for input, want := range tests {
		phase, health := Legacy(input)
		if phase != want.phase || health != want.health {
			t.Errorf("Legacy(%q) = %s/%s, want %s/%s", input, phase, health, want.phase, want.health)
		}
	}
}

func TestHealthValidationAndCompatibilityStatus(t *testing.T) {
	for _, health := range []Health{HealthUnknown, HealthHealthy, HealthUnhealthy} {
		if !health.Valid() {
			t.Errorf("known health %q is invalid", health)
		}
	}
	if Health("bogus").Valid() {
		t.Fatal("unknown health must be invalid")
	}
	if got := CompatibilityStatus(PhaseRunning, HealthHealthy); got != "healthy" {
		t.Fatalf("healthy running compatibility status = %q", got)
	}
	if got := CompatibilityStatus(PhaseRunning, HealthUnhealthy); got != "unhealthy" {
		t.Fatalf("unhealthy running compatibility status = %q", got)
	}
	if got := CompatibilityStatus(PhaseStopping, HealthHealthy); got != "stopping" {
		t.Fatalf("non-running compatibility status = %q", got)
	}
}
