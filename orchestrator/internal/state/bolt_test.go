package state

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBoltStoreRoundTrip(t *testing.T) {
	store, err := NewBoltStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	val, err := store.Get(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil for missing key, got %q", val)
	}

	if err := store.Put(ctx, "key1", []byte("value1")); err != nil {
		t.Fatal(err)
	}
	val, err = store.Get(ctx, "key1")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "value1" {
		t.Fatalf("expected value1, got %q", val)
	}
}

func TestBoltStoreListPrefix(t *testing.T) {
	store, err := NewBoltStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	store.Put(ctx, "trellis/default/jobs/web", []byte("web"))
	store.Put(ctx, "trellis/default/jobs/api", []byte("api"))
	store.Put(ctx, "trellis/default/nodes/n1", []byte("node1"))
	store.Put(ctx, "trellis/other/jobs/x", []byte("x"))

	result, err := store.List(ctx, "trellis/default/jobs/")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(result), result)
	}
	if string(result["trellis/default/jobs/web"]) != "web" {
		t.Fatal("missing web entry")
	}
	if string(result["trellis/default/jobs/api"]) != "api" {
		t.Fatal("missing api entry")
	}
}

func TestBoltStoreDelete(t *testing.T) {
	store, err := NewBoltStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	store.Put(ctx, "key", []byte("value"))
	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	val, err := store.Get(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil after delete, got %q", val)
	}

	if err := store.Delete(ctx, "nonexistent"); err != nil {
		t.Fatal("delete of missing key should not error")
	}
}

var _ StateStore = (*BoltStore)(nil)
