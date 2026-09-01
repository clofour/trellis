package main

import (
	"strings"
	"testing"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
)

func TestDiffJobSpecsUsesNamesAndHumanDurations(t *testing.T) {
	before := &spec.JobSpec{Name: "web", Namespace: "default", TaskGroups: []spec.TaskGroupSpec{{
		Name: "frontend", Count: 1, Restart: &spec.RestartPolicySpec{MaxRestarts: 3, Window: time.Minute},
		Tasks: []spec.TaskSpec{{Name: "app", Image: "example/app:v1"}},
	}}}
	after := &spec.JobSpec{Name: "web", Namespace: "default", TaskGroups: []spec.TaskGroupSpec{{
		Name: "frontend", Count: 1, Restart: &spec.RestartPolicySpec{MaxRestarts: 3, Window: 2 * time.Minute},
		Tasks: []spec.TaskSpec{{Name: "app", Image: "example/app:v2"}},
	}}}
	changes := diffJobSpecs(before, after)
	if len(changes) != 2 {
		t.Fatalf("got %d changes: %#v", len(changes), changes)
	}
	var paths []string
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "task_groups[frontend].tasks[app].image") {
		t.Fatalf("named task path missing: %s", joined)
	}
	if !strings.Contains(joined, "task_groups[frontend].restart.window") {
		t.Fatalf("duration path missing: %s", joined)
	}
	for _, change := range changes {
		if strings.HasSuffix(change.Path, ".window") && formatChangeValue(change.Path, change.After) != "2m0s" {
			t.Fatalf("duration formatted as %q", formatChangeValue(change.Path, change.After))
		}
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
}
