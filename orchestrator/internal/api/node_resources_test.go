package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/clofour/trellis/internal/nodecapacity"
	"github.com/google/uuid"
)

func TestNodeRegistrationUsesAllocatableResourcesOnWire(t *testing.T) {
	reservedCPU := 500
	reservedMemory := int64(1 << 30)
	t.Cleanup(func() { _ = nodecapacity.ConfigureReserve(nil, nil) })
	if err := nodecapacity.ConfigureReserve(&reservedCPU, &reservedMemory); err != nil {
		t.Fatal(err)
	}

	id := uuid.New()
	raw, err := json.Marshal(NodeRegistrationRequest{ID: id, CPU: 8000, Memory: 32 << 30})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["cpu"] != float64(7500) || fields["cpu_capacity"] != float64(8000) || fields["cpu_allocatable"] != float64(7500) {
		t.Fatalf("unexpected CPU fields: %#v", fields)
	}
	if fields["memory"] != float64(31<<30) || fields["memory_capacity"] != float64(32<<30) || fields["memory_allocatable"] != float64(31<<30) {
		t.Fatalf("unexpected memory fields: %#v", fields)
	}

	var decoded NodeRegistrationRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.CPU != 7500 || decoded.Memory != 31<<30 {
		t.Fatalf("server-facing resources = %d/%d", decoded.CPU, decoded.Memory)
	}
}

func TestNodeResponseExposesResourceMetadata(t *testing.T) {
	id := uuid.New()
	usage := 0.42
	used := int64(8 << 30)
	available := int64(24 << 30)
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	storeNodeResourceState(id, nodeResourceState{
		CPUCapacity: 8000, MemoryCapacity: 32 << 30,
		CPUAllocatable: 7500, MemoryAllocatable: 31 << 30,
		CPUUsage: &usage, MemoryUsed: &used, MemoryAvailable: &available, MetricsAt: &at,
	})

	raw, err := json.Marshal(NodeResponse{ID: id, CPU: 7500, Memory: 31 << 30})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["cpu_capacity"] != float64(8000) || fields["cpu_allocatable"] != float64(7500) || fields["cpu_usage"] != 0.42 {
		t.Fatalf("unexpected CPU status: %#v", fields)
	}
	if fields["memory_capacity"] != float64(32<<30) || fields["memory_allocatable"] != float64(31<<30) || fields["memory_used"] != float64(8<<30) {
		t.Fatalf("unexpected memory status: %#v", fields)
	}
}
