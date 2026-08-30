package server

import (
	"context"
	"testing"

	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
	"github.com/clofour/trellis/internal/state"
)

type memoryStore map[string][]byte

func (m memoryStore) Get(_ context.Context, key string) ([]byte, error) { return m[key], nil }
func (m memoryStore) List(_ context.Context, prefix string) (map[string][]byte, error) {
	result := map[string][]byte{}
	for key, value := range m {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key] = value
		}
	}
	return result, nil
}
func (m memoryStore) Put(_ context.Context, key string, value []byte) error { m[key] = value; return nil }
func (m memoryStore) Delete(_ context.Context, key string) error           { delete(m, key); return nil }

var _ state.StateStore = memoryStore{}

func TestStateControllerRoundTripsDurableLeaderState(t *testing.T) {
	ctx := context.Background()
	controller := NewStateController(memoryStore{}, "test")
	job := &Job{Spec: &spec.JobSpec{Name: "web"}, Revision: 3}
	if err := controller.PutJob(ctx, "web", job); err != nil {
		t.Fatal(err)
	}
	jobs, err := controller.ListJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if jobs["web"] == nil || jobs["web"].Revision != 3 {
		t.Fatalf("unexpected jobs: %#v", jobs)
	}
	allocation := &Allocation{ID: "web-1", JobName: "web", JobRevision: 3, Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy}
	if err := controller.PutAllocation(ctx, allocation); err != nil {
		t.Fatal(err)
	}
	allocations, err := controller.ListAllocations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if allocations["web-1"] == nil || allocations["web-1"].JobRevision != 3 {
		t.Fatalf("unexpected allocations: %#v", allocations)
	}
	if err := controller.DeleteAllocation(ctx, "web-1"); err != nil {
		t.Fatal(err)
	}
	allocations, err = controller.ListAllocations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allocations) != 0 {
		t.Fatalf("allocation was not deleted: %#v", allocations)
	}
}

func TestStateControllerLoadsLegacyAllocationRecord(t *testing.T) {
	store := memoryStore{
		"trellis/test/allocations/legacy": []byte(`{"Namespace":"default","JobName":"web","TaskGroupName":"app","name":"legacy","revision":2,"Task":{"name":"task","image":"nginx"},"status":"unhealthy"}`),
	}
	controller := NewStateController(store, "test")
	allocations, err := controller.ListAllocations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	allocation := allocations["legacy"]
	if allocation == nil {
		t.Fatal("legacy allocation was not loaded")
	}
	if allocation.ID != "legacy" || allocation.JobRevision != 2 || len(allocation.Tasks) != 1 {
		t.Fatalf("legacy fields were not normalized: %#v", allocation)
	}
	if allocation.Phase != lifecycle.PhaseRunning || allocation.Health != lifecycle.HealthUnhealthy {
		t.Fatalf("legacy status was not translated: %s/%s", allocation.Phase, allocation.Health)
	}
}
