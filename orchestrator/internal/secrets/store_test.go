package secrets

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/clofour/trellis/internal/state"
)

func TestStoreIsWriteOnlyAndEncryptedAtRest(t *testing.T) {
	bolt, err := state.NewBoltStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer bolt.Close()
	store, err := NewStore(bolt, "cluster", "key-1", bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	value := []byte("sentinel-plaintext-value")
	zero := uint64(0)
	meta, err := store.Set(ctx, "acme", "database", value, &zero)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Version != 1 || meta.Name != "database" {
		t.Fatalf("unexpected metadata: %#v", meta)
	}
	raw, err := bolt.List(ctx, "trellis/cluster/secrets/")
	if err != nil {
		t.Fatal(err)
	}
	for _, encoded := range raw {
		if bytes.Contains(encoded, value) {
			t.Fatal("persisted record contains plaintext")
		}
	}
	got, version, err := store.Resolve(ctx, "acme", "database")
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 || !bytes.Equal(got, value) {
		t.Fatalf("resolved %q version %d", got, version)
	}
	wrong := uint64(0)
	if _, err := store.Set(ctx, "acme", "database", []byte("new"), &wrong); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	one := uint64(1)
	if _, err := store.Set(ctx, "acme", "database", []byte("new"), &one); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "acme", "database"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetMetadata(ctx, "acme", "database"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestStoreRejectsWrongKeyAndOversizedValues(t *testing.T) {
	bolt, err := state.NewBoltStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer bolt.Close()
	ctx := context.Background()
	store, _ := NewStore(bolt, "cluster", "key-1", bytes.Repeat([]byte{1}, 32))
	if _, err := store.Set(ctx, "acme", "secret", make([]byte, MaxValueSize+1), nil); err == nil {
		t.Fatal("expected size error")
	}
	if _, err := store.Set(ctx, "acme", "secret", []byte("value"), nil); err != nil {
		t.Fatal(err)
	}
	other, _ := NewStore(bolt, "cluster", "key-2", bytes.Repeat([]byte{2}, 32))
	if _, _, err := other.Resolve(ctx, "acme", "secret"); err == nil {
		t.Fatal("expected unavailable key error")
	}
}
