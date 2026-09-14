package agent

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/clofour/trellis/internal/spec"
)

func TestVolumeManagerRejectsTraversal(t *testing.T) {
	manager := NewVolumeManager(t.TempDir())
	invalid := []string{"@/../data", "@/data/../other", "relative/path"}
	for _, hostPath := range invalid {
		if _, err := manager.Create("ns", "job", "task", spec.VolumeSpec{Name: "data", HostPath: hostPath, ContainerPath: "/data"}); err == nil {
			t.Errorf("Create(%q) succeeded", hostPath)
		}
	}
}

func TestVolumeManagerCreatesNamespaceScopedAliasPath(t *testing.T) {
	root := t.TempDir()
	manager := NewVolumeManager(root)
	mount, err := manager.Create("blog", "mysql", "db", spec.VolumeSpec{Name: "data", HostPath: "@/mysql/data", ContainerPath: "/var/lib/mysql"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	want := filepath.Join(root, "volumes", "namespaces", "blog", "mysql", "data")
	if mount.HostPath != want || mount.ContainerPath != "/var/lib/mysql" {
		t.Errorf("unexpected mount: %#v", mount)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("directory not created: %v", err)
	}
	if got := manager.AvailableHostVolumes(); !slices.Contains(got, "blog/data") {
		t.Fatalf("volume registration not advertised: %v", got)
	}
}

func TestVolumeManagerUsesExplicitHostPath(t *testing.T) {
	root := t.TempDir()
	host := filepath.Join(root, "postgres")
	if err := os.Mkdir(host, 0o750); err != nil {
		t.Fatal(err)
	}
	manager := NewVolumeManager(root)
	mount, err := manager.Create("default", "app", "db", spec.VolumeSpec{Name: "database", HostPath: host, ContainerPath: "/data", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != host || !mount.ReadOnly {
		t.Fatalf("unexpected mount: %#v", mount)
	}
	if got := manager.AvailableHostVolumes(); !slices.Contains(got, "default/database") {
		t.Fatalf("explicit path registration not advertised: %v", got)
	}
}

func TestVolumeManagerPersistsRegistrations(t *testing.T) {
	root := t.TempDir()
	manager := NewVolumeManager(root)
	if _, err := manager.Create("acme", "app", "task", spec.VolumeSpec{Name: "uploads", HostPath: "@/uploads", ContainerPath: "/uploads"}); err != nil {
		t.Fatal(err)
	}
	reloaded := NewVolumeManager(root)
	if got := reloaded.AvailableHostVolumes(); !slices.Contains(got, "acme/uploads") {
		t.Fatalf("registration did not survive reload: %v", got)
	}
}
