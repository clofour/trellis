package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageRejectsEscapingKeys(t *testing.T) {
	root := t.TempDir()
	store := NewLocalStorage(root)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "../outside", filepath.Join("..", "outside"), filepath.Join(root, "absolute")} {
		if err := store.Put(key, "value"); err == nil {
			t.Errorf("Put(%q) succeeded", key)
		}
		if err := store.Delete(key); err == nil {
			t.Errorf("Delete(%q) succeeded", key)
		}
	}
}

func TestLocalStoragePutIsPrivateAndSupportsNestedKeys(t *testing.T) {
	store := NewLocalStorage(t.TempDir())
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("nested/token", "secret"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.formatPath("nested/token"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	var value string
	if err := store.Get("nested/token", &value); err != nil {
		t.Fatal(err)
	}
	if value != "secret" {
		t.Fatalf("value = %q", value)
	}
}
