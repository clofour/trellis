package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
)

// VolumeManager resolves and prepares task volume mounts.
type VolumeManager struct {
	dataRootPath string
	hostVolumes  map[string]string
}

// NewVolumeManager creates a volume manager.
func NewVolumeManager(dataRoot ...string) *VolumeManager {
	root := "/var/lib/trellis/data"
	if len(dataRoot) > 0 && dataRoot[0] != "" {
		root = dataRoot[0]
	}
	return &VolumeManager{
		dataRootPath: root,
		hostVolumes:  make(map[string]string),
	}
}

// SetHostVolumes is retained temporarily for node-config compatibility. Named
// host-volume capabilities are no longer used for placement; volume locality is
// established by namespace/name registration when an allocation is placed.
func (vm *VolumeManager) SetHostVolumes(volumes map[string]string) { vm.hostVolumes = volumes }

// AvailableHostVolumes is retained temporarily for node-registration compatibility.
func (vm *VolumeManager) AvailableHostVolumes() []string {
	available := make([]string, 0, len(vm.hostVolumes))
	for name, path := range vm.hostVolumes {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			available = append(available, name)
		}
	}
	slices.Sort(available)
	return available
}

// Create resolves and prepares an allocation volume mount. @/ is a path prefix
// for the current namespace below Trellis's volume root; it does not change the
// logical identity of the volume. Absolute host paths are operator-managed and
// must already exist.
func (vm *VolumeManager) Create(namespace string, _ string, _ string, volume spec.VolumeSpec) (*runtime.Mount, error) {
	hostPath, managed, err := vm.resolveHostPath(namespace, volume.HostPath)
	if err != nil {
		return nil, err
	}
	if managed {
		if err := os.MkdirAll(hostPath, 0o750); err != nil {
			return nil, fmt.Errorf("creating volume dir %s: %w", hostPath, err)
		}
	} else {
		info, err := os.Stat(hostPath)
		if err != nil {
			return nil, fmt.Errorf("checking host path %s: %w", hostPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("host path %s is not a directory", hostPath)
		}
	}
	return &runtime.Mount{HostPath: hostPath, ContainerPath: volume.ContainerPath, ReadOnly: volume.ReadOnly}, nil
}

// Check reports whether a volume backing directory is available.
func (vm *VolumeManager) Check(namespace string, _ string, _ string, volume spec.VolumeSpec) (bool, error) {
	hostPath, _, err := vm.resolveHostPath(namespace, volume.HostPath)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(hostPath)
	if err != nil {
		return false, fmt.Errorf("checking volume dir %s: %w", hostPath, err)
	}
	return info.IsDir(), nil
}

// Delete removes a Trellis-rooted (@/) backing directory. Explicit absolute
// host paths are operator-owned and are never deleted by Trellis.
func (vm *VolumeManager) Delete(namespace string, _ string, _ string, volume spec.VolumeSpec) error {
	hostPath, managed, err := vm.resolveHostPath(namespace, volume.HostPath)
	if err != nil {
		return err
	}
	if !managed {
		return nil
	}
	if err := os.RemoveAll(hostPath); err != nil {
		return fmt.Errorf("deleting volume dir %s: %w", hostPath, err)
	}
	return nil
}

func (vm *VolumeManager) resolveHostPath(namespace, hostPath string) (string, bool, error) {
	if hostPath == "" {
		return "", false, fmt.Errorf("host path is required")
	}
	if !strings.HasPrefix(hostPath, "@/") {
		if !filepath.IsAbs(hostPath) || filepath.Clean(hostPath) != hostPath {
			return "", false, fmt.Errorf("host path %q must be a clean absolute path or begin with @/", hostPath)
		}
		return hostPath, false, nil
	}
	if namespace == "" || namespace == "." || namespace == ".." || filepath.Base(namespace) != namespace || strings.ContainsAny(namespace, `/\\`) {
		return "", false, fmt.Errorf("invalid namespace path component %q", namespace)
	}
	rel := strings.TrimPrefix(hostPath, "@/")
	if rel == "" || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("invalid Trellis volume path %q", hostPath)
	}
	return filepath.Join(vm.dataRootPath, "volumes", "namespaces", namespace, filepath.FromSlash(rel)), true, nil
}
