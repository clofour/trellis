package main

import (
	"strings"
	"testing"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/plan"
)

func TestPrintJobPlanFormatsHumanDurations(t *testing.T) {
	previousOutput := config.Output
	config.Output = "table"
	defer func() { config.Output = previousOutput }()

	result := &plan.Result{
		Action:       "update",
		Namespace:    "default",
		Job:          "web",
		BaseRevision: 7,
		Changes: []plan.Change{{
			Operation: "change",
			Path:      "task_groups[frontend].restart.window",
			Before:    float64(60_000_000_000),
			After:     float64(120_000_000_000),
		}},
	}
	var out strings.Builder
	if err := printJobPlan(&out, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1m0s -> 2m0s") {
		t.Fatalf("plan output did not humanize duration: %q", out.String())
	}
}

func TestResolveAllocationPrefix(t *testing.T) {
	allocations := []api.AllocationResponse{{ID: "abcdef12-one"}, {ID: "12345678-two"}}
	got, err := resolveAllocationPrefix(allocations, "abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "abcdef12-one" {
		t.Fatalf("resolved %q", got.ID)
	}
	_, err = resolveAllocationPrefix(append(allocations, api.AllocationResponse{ID: "abcdef99-three"}), "abcdef")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestJobStateSeparatesConvergingAndDegraded(t *testing.T) {
	converging := &api.JobStatusResponse{Desired: 2, Running: 1, Healthy: 1, Allocations: []api.AllocationResponse{{Phase: lifecycle.PhaseStarting, Health: lifecycle.HealthUnknown}}}
	if got := jobState(converging); got != "converging" {
		t.Fatalf("state = %q", got)
	}
	degraded := &api.JobStatusResponse{Desired: 2, Running: 2, Healthy: 1, Allocations: []api.AllocationResponse{{Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthUnhealthy}}}
	if got := jobState(degraded); got != "degraded" {
		t.Fatalf("state = %q", got)
	}
	ready := &api.JobStatusResponse{Desired: 2, Running: 2, Healthy: 2}
	if got := jobState(ready); got != "ready" {
		t.Fatalf("state = %q", got)
	}
	overlap := &api.JobStatusResponse{
		Revision: 2,
		Desired:  2,
		Running:  3,
		Healthy:  3,
		Allocations: []api.AllocationResponse{
			{JobRevision: 1, Draining: true, Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy},
			{JobRevision: 2, Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy},
			{JobRevision: 2, Phase: lifecycle.PhaseStarting, Health: lifecycle.HealthUnknown},
		},
	}
	if got := jobState(overlap); got != "converging" {
		t.Fatalf("rolling overlap state = %q, want converging", got)
	}
}
