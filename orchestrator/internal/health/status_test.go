package health

import "testing"

func TestTaskHealthUsesConfiguredThreshold(t *testing.T) {
	health := NewTaskHealth(2)
	if changed, status := health.RecordResult(true); changed || status != StatusInitializing {
		t.Fatalf("first pass = (%v, %q), want unchanged initializing", changed, status)
	}
	if changed, status := health.RecordResult(true); !changed || status != StatusHealthy {
		t.Fatalf("second pass = (%v, %q), want changed healthy", changed, status)
	}
	if changed, status := health.RecordResult(false); changed || status != StatusHealthy {
		t.Fatalf("first failure = (%v, %q), want unchanged healthy", changed, status)
	}
	if changed, status := health.RecordResult(false); !changed || status != StatusUnhealthy {
		t.Fatalf("second failure = (%v, %q), want changed unhealthy", changed, status)
	}
}

func TestTaskHealthUsesDefaultThresholdForZero(t *testing.T) {
	health := NewTaskHealth(0)
	for i := 0; i < defaultCheckThreshold-1; i++ {
		if changed, _ := health.RecordResult(true); changed {
			t.Fatalf("status changed after %d passes", i+1)
		}
	}
	if changed, status := health.RecordResult(true); !changed || status != StatusHealthy {
		t.Fatalf("default threshold result = (%v, %q), want changed healthy", changed, status)
	}
}
