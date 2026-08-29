package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDNSConfigCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "allocation-resolv.conf")
	if err := writeDNSConfig(path, []string{"127.0.0.1:8053"}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "nameserver 127.0.0.1:8053\n"; string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
