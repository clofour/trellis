package server

import (
	"encoding/json"
	"testing"

	"github.com/clofour/trellis/internal/lifecycle"
)

func TestLegacyAllocationStatusLoadsSafely(t *testing.T) {
	for legacy, expected := range map[string]struct {
		phase  lifecycle.Phase
		health lifecycle.Health
	}{
		"pending":   {lifecycle.PhasePlaced, lifecycle.HealthUnknown},
		"healthy":   {lifecycle.PhaseRunning, lifecycle.HealthHealthy},
		"unhealthy": {lifecycle.PhaseRunning, lifecycle.HealthUnhealthy},
	} {
		var allocation Allocation
		if err := json.Unmarshal([]byte(`{"name":"legacy","revision":2,"status":"`+legacy+`"}`), &allocation); err != nil {
			t.Fatal(err)
		}
		if allocation.ID != "legacy" || allocation.Generation != 1 || allocation.JobRevision != 2 || allocation.Phase != expected.phase || allocation.Health != expected.health {
			t.Fatalf("legacy %s decoded as id=%s gen=%d rev=%d phase=%s health=%s", legacy, allocation.ID, allocation.Generation, allocation.JobRevision, allocation.Phase, allocation.Health)
		}
	}
}

func TestRetryDelayIsDeterministicAndBounded(t *testing.T) {
	first := retryDelay("allocation", 4)
	if first != retryDelay("allocation", 4) {
		t.Fatal("retry jitter is not deterministic")
	}
	if first < 8_000_000_000 || first >= 8_500_000_000 {
		t.Fatalf("unexpected retry delay %s", first)
	}
}
