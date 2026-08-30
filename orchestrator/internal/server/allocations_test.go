package server

import (
	"testing"

	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

func TestListAllocationsWithFilters(t *testing.T) {
	acmeNode := &Node{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Host: "10.0.0.1"}
	stagingNode := &Node{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Host: "10.0.1.1"}

	s := &Server{
		jobs: map[string]*Job{
			jobKey("acme", "web"): {
				Spec: &spec.JobSpec{Namespace: "acme", Name: "web", TaskGroups: []spec.TaskGroupSpec{{Name: "frontend", Labels: map[string]string{"trellis.expose": "true", "trellis/domain": "example.com"}}}},
			},
			jobKey("acme", "db"): {
				Spec: &spec.JobSpec{Namespace: "acme", Name: "db", TaskGroups: []spec.TaskGroupSpec{{Name: "primary", Labels: map[string]string{"trellis/engine": "postgres"}}}},
			},
			jobKey("staging", "web"): {
				Spec: &spec.JobSpec{Namespace: "staging", Name: "web", TaskGroups: []spec.TaskGroupSpec{{Name: "frontend", Labels: map[string]string{"trellis.expose": "true"}}}},
			},
		},
		allocations: []*Allocation{
			{Namespace: "acme", JobName: "web", TaskGroupName: "frontend", Name: "acme-web-1", Status: AllocationStatusHealthy, Node: acmeNode},
			{Namespace: "acme", JobName: "db", TaskGroupName: "primary", Name: "acme-db-1", Status: AllocationStatusHealthy, Node: acmeNode},
			{Namespace: "staging", JobName: "web", TaskGroupName: "frontend", Name: "staging-web-1", Status: AllocationStatusHealthy, Node: stagingNode},
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
