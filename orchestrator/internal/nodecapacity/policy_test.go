package nodecapacity

import "testing"

func TestResolveUsesSensibleDefaults(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureReserve(nil, nil) })
	if err := ConfigureReserve(nil, nil); err != nil {
		t.Fatal(err)
	}

	cpu, memory, err := Resolve(8000, 32<<30)
	if err != nil {
		t.Fatal(err)
	}
	if cpu != 7600 {
		t.Fatalf("allocatable CPU = %d, want 7600", cpu)
	}
	if memory != (32<<30)-(32<<30)/20 {
		t.Fatalf("allocatable memory = %d, want %d", memory, (32<<30)-(32<<30)/20)
	}
}

func TestResolveUsesPerResourceOverrides(t *testing.T) {
	reservedCPU := 500
	reservedMemory := int64(1 << 30)
	t.Cleanup(func() { _ = ConfigureReserve(nil, nil) })
	if err := ConfigureReserve(&reservedCPU, &reservedMemory); err != nil {
		t.Fatal(err)
	}

	cpu, memory, err := Resolve(8000, 32<<30)
	if err != nil {
		t.Fatal(err)
	}
	if cpu != 7500 || memory != 31<<30 {
		t.Fatalf("allocatable = %dm/%d, want 7500m/%d", cpu, memory, int64(31<<30))
	}
}

func TestResolveRejectsReserveLargerThanCapacity(t *testing.T) {
	reservedCPU := 2000
	t.Cleanup(func() { _ = ConfigureReserve(nil, nil) })
	if err := ConfigureReserve(&reservedCPU, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Resolve(1000, 1<<30); err == nil {
		t.Fatal("expected reserve larger than capacity to fail")
	}
}
