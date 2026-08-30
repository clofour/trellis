package lifecycle

import "testing"

func TestTransitions(t *testing.T) {
	if err := Transition(PhasePlaced, PhaseStarting); err != nil {
		t.Fatal(err)
	}
	if err := Transition(PhaseRunning, PhasePlaced); err == nil {
		t.Fatal("running -> placed must be rejected")
	}
}

func TestLegacyStatus(t *testing.T) {
	phase, health := Legacy("unhealthy")
	if phase != PhaseRunning || health != HealthUnhealthy {
		t.Fatalf("got %s/%s", phase, health)
	}
}
