package server

import (
	"testing"
	"time"

	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

func TestListAllocationsWithFilters(t *testing.T) {
	acmeNode := &Node{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Host: "10.0.0.1"}
	stagingNode := &Node{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Host: "10.0.1.1"}

	s := &Server{
		now: time.Now,
		jobs: map[string]*Job{
			jobKey("acme", "web"): {Spec: &spec.JobSpec{Namespace: "acme", Name: "web", TaskGroups: []spec.TaskGroupSpec{{Name: "frontend", Labels: map[string]string{"trellis.expose": "true", "trellis/domain": "example.com"}}}}},
			jobKey("acme", "db"): {Spec: &spec.JobSpec{Namespace: "acme", Name: "db", TaskGroups: []spec.TaskGroupSpec{{Name: "primary", Labels: map[string]string{"trellis/engine": "postgres"}}}}},
			jobKey("staging", "web"): {Spec: &spec.JobSpec{Namespace: "staging", Name: "web", TaskGroups: []spec.TaskGroupSpec{{Name: "frontend", Labels: map[string]string{"trellis.expose": "true"}}}}},
		},
		allocations: []*Allocation{
			{Namespace: "acme", JobName: "web", TaskGroupName: "frontend", ID: "acme-web-1", Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy, Node: acmeNode},
			{Namespace: "acme", JobName: "db", TaskGroupName: "primary", ID: "acme-db-1", Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy, Node: acmeNode},
			{Namespace: "staging", JobName: "web", TaskGroupName: "frontend", ID: "staging-web-1", Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy, Node: stagingNode},
		},
	}

	if got := s.ListAllocations("acme", nil); len(got) != 2 {
		t.Fatalf("expected 2 acme allocations, got %d", len(got))
	}
	if got := s.ListAllocations("acme", &AllocationListFilter{Job: "web"}); len(got) != 1 || got[0].Job != "web" {
		t.Fatalf("expected one web allocation, got %#v", got)
	}
	if got := s.ListAllocations("acme", &AllocationListFilter{Label: "trellis.expose:true"}); len(got) != 1 || got[0].Labels["trellis/domain"] != "example.com" {
		t.Fatalf("expected exposed allocation with domain label, got %#v", got)
	}
	if got := s.ListAllocations("acme", &AllocationListFilter{Label: "trellis/engine"}); len(got) != 1 || got[0].Job != "db" {
		t.Fatalf("expected db allocation by label existence, got %#v", got)
	}
	if got := s.ListAllocations("", &AllocationListFilter{Job: "web", Label: "trellis.expose:true"}); len(got) != 2 {
		t.Fatalf("expected two exposed web allocations across namespaces, got %d", len(got))
	}
}

func TestAllocationEvents(t *testing.T) {
	now := time.Now().UTC()
	alloc := &Allocation{Namespace: "acme", JobName: "web", TaskGroupName: "frontend", ID: "acme-web-1", Phase: lifecycle.PhasePlaced}
	alloc.normalize(now)
	s := &Server{now: time.Now, jobs: map[string]*Job{}, allocations: []*Allocation{alloc}}

	events, ok := s.AllocationEvents("acme", "acme-web-1")
	if !ok || len(events) != 0 {
		t.Fatalf("expected empty event list, got %#v, %v", events, ok)
	}
	if err := alloc.Transition(lifecycle.PhaseStarting, now, "scheduled", ""); err != nil {
		t.Fatal(err)
	}
	if err := alloc.Transition(lifecycle.PhaseRunning, now.Add(time.Second), "", ""); err != nil {
		t.Fatal(err)
	}
	if err := alloc.Transition(lifecycle.PhaseFailed, now.Add(2*time.Second), "oom", "container exited"); err != nil {
		t.Fatal(err)
	}
	events, ok = s.AllocationEvents("acme", "acme-web-1")
	if !ok || len(events) != 3 {
		t.Fatalf("expected 3 events, got %#v, %v", events, ok)
	}
	if events[0].Phase != lifecycle.PhaseStarting || events[0].Reason != "scheduled" {
		t.Errorf("event 0: unexpected %+v", events[0])
	}
	if events[2].Phase != lifecycle.PhaseFailed || events[2].Reason != "oom" || events[2].Message != "container exited" {
		t.Errorf("event 2: unexpected %+v", events[2])
	}
	if _, ok := s.AllocationEvents("staging", "acme-web-1"); ok {
		t.Fatal("expected not-found for wrong namespace")
	}
}
