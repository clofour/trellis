package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clofour/trellis/internal/spec"
)

func TestVolumeManagerRejectsTraversal(t *testing.T) {
	manager := NewVolumeManager(t.TempDir())
	invalid := []struct{ ns, job, task, volume string }{
		{"../ns", "job", "task", "data"},
		{"ns", "../job", "task", "data"},
		{"ns", "job", "../task", "data"},
		{"ns", "job", "task", "../data"},
	}
	for _, tc := range invalid {
		if _, err := manager.Create(tc.ns, tc.job, tc.task, spec.VolumeSpec{Name: tc.volume, Path: "/data"}); err == nil {
			t.Errorf("Create(%q, %q, %q, %q) succeeded", tc.ns, tc.job, tc.task, tc.volume)
		}
	}
}

func TestVolumeManagerCreatePath(t *testing.T) {
	root := t.TempDir()
	manager := NewVolumeManager(root)
	mount, err := manager.Create("blog", "mysql", "db", spec.VolumeSpec{Name: "data", Path: "/var/lib/mysql"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	want := filepath.Join(root, "namespaces", "blog", "mysql", "db", "data")
	if mount.HostPath != want {
		t.Errorf("HostPath = %q, want %q", mount.HostPath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}
