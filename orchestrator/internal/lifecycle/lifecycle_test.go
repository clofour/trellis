package lifecycle

import "testing"

func TestTransitions(t *testing.T) {
	phases := []Phase{PhasePlaced, PhaseStarting, PhaseRunning, PhaseStopping, PhaseStopped, PhaseFailed, PhaseLost}
	allowed := map[Phase]map[Phase]bool{
		PhasePlaced:   {PhaseStarting: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
		PhaseStarting: {PhaseRunning: true, PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
		PhaseRunning:  {PhaseStopping: true, PhaseFailed: true, PhaseLost: true},
		PhaseStopping: {PhaseStopped: true, PhaseFailed: true, PhaseLost: true},
		PhaseStopped:  {PhaseStarting: true},
		PhaseFailed:   {PhaseStarting: true, PhaseStopping: true, PhaseLost: true},
		PhaseLost:     {PhaseStarting: true, PhaseStopping: true, PhaseStopped: true},
	}
	for _, from := range phases {
		for _, to := range phases {
			want := from == to || allowed[from][to]
			if got := Transition(from, to) == nil; got != want {
				t.Errorf("Transition(%q, %q) success = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestTransitionRejectsUnknownPhases(t *testing.T) {
	for _, test := range []struct{ from, to Phase }{{"", PhasePlaced}, {PhasePlaced, "unknown"}, {"unknown", "unknown"}} {
		if err := Transition(test.from, test.to); err == nil {
			t.Errorf("Transition(%q, %q) unexpectedly succeeded", test.from, test.to)
		}
	}
}

func TestLegacyStatus(t *testing.T) {
	tests := []struct {
		status string
		phase  Phase
		health Health
	}{
		{"healthy", PhaseRunning, HealthHealthy}, {"unhealthy", PhaseRunning, HealthUnhealthy},
		{"running", PhaseRunning, HealthUnknown}, {"stopped", PhaseStopped, HealthUnknown},
		{"failed", PhaseFailed, HealthUnknown}, {"lost", PhaseLost, HealthUnknown},
		{"pending", PhasePlaced, HealthUnknown}, {"unknown", PhasePlaced, HealthUnknown},
	}
	for _, test := range tests {
		phase, health := Legacy(test.status)
		if phase != test.phase || health != test.health {
			t.Errorf("Legacy(%q) = %s/%s, want %s/%s", test.status, phase, health, test.phase, test.health)
		}
	}
}

func TestCompatibilityStatus(t *testing.T) {
	tests := []struct {
		phase  Phase
		health Health
		want   string
	}{
		{PhaseRunning, HealthHealthy, "healthy"}, {PhaseRunning, HealthUnhealthy, "unhealthy"},
		{PhaseRunning, HealthUnknown, "running"}, {PhaseStopped, HealthHealthy, "stopped"},
	}
	for _, test := range tests {
		if got := CompatibilityStatus(test.phase, test.health); got != test.want {
			t.Errorf("CompatibilityStatus(%q, %q) = %q, want %q", test.phase, test.health, got, test.want)
		}
	}
}
