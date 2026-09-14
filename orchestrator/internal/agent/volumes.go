package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
)

// VolumeManager resolves task volume mounts and persists the namespace-scoped
// volume registrations that this node owns.
type VolumeManager struct {
	dataRootPath  string
	mu            sync.RWMutex
	registrations map[string]string
}

// NewVolumeManager creates a volume manager.
func NewVolumeManager(dataRoot ...string) *VolumeManager {
	root := "/var/lib/trellis/data"
	if len(dataRoot) > 0 && dataRoot[0] != "" {
		root = dataRoot[0]
	}
	vm := &VolumeManager{dataRootPath: root, registrations: make(map[string]string)}
	_ = vm.loadRegistrations()
	return vm
}

// AvailableHostVolumes returns persisted volume registrations. The existing
// node-registration field name is retained for wire compatibility; entries are
// namespace/name identities rather than configured host-volume capabilities.
func (vm *VolumeManager) AvailableHostVolumes() []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	available := make([]string, 0, len(vm.registrations))
	for key := range vm.registrations {
		available = append(available, key)
	}
	slices.Sort(available)
	return available
}

// Create resolves and prepares an allocation volume mount. @/ is a path prefix
// for the current namespace below Trellis's volume root. Absolute host paths are
// operator-managed and must already exist. Once prepared, the namespace/name is
// persistently registered to this node; later path changes keep the same identity.
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
	if err := vm.register(namespace, volume.Name, hostPath); err != nil {
		return nil, err
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

// Delete intentionally does not remove a registration or its data. A named
// volume is namespace-scoped state whose lifetime is independent of an allocation.
func (vm *VolumeManager) Delete(_ string, _ string, _ string, _ spec.VolumeSpec) error { return nil }

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

func volumeRegistrationName(namespace, name string) string { return namespace + "/" + name }

func (vm *VolumeManager) register(namespace, name, hostPath string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("volume namespace and name are required")
	}
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.registrations[volumeRegistrationName(namespace, name)] = hostPath
	return vm.persistRegistrationsLocked()
}

func (vm *VolumeManager) registrationsPath() string {
	return filepath.Join(vm.dataRootPath, "volume-registrations.json")
}

func (vm *VolumeManager) loadRegistrations() error {
	raw, err := os.ReadFile(vm.registrationsPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var registrations map[string]string
	if err := json.Unmarshal(raw, &registrations); err != nil {
		return err
	}
	if registrations != nil {
		vm.registrations = registrations
	}
	return nil
}

func (vm *VolumeManager) persistRegistrationsLocked() error {
	if err := os.MkdirAll(vm.dataRootPath, 0o750); err != nil {
		return fmt.Errorf("creating data root: %w", err)
	}
	raw, err := json.MarshalIndent(vm.registrations, "", "  ")
	if err != nil {
		return err
	}
	path := vm.registrationsPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("writing volume registrations: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("committing volume registrations: %w", err)
	}
	return nil
}
