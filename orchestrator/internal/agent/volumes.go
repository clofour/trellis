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

// VolumeManager manages allocation volume mounts.
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

// SetHostVolumes configures opaque, operator-managed host-volume identities.
func (vm *VolumeManager) SetHostVolumes(volumes map[string]string) { vm.hostVolumes = volumes }

// AvailableHostVolumes returns configured volumes whose root currently exists.
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

// Create resolves and prepares an allocation volume mount.
func (vm *VolumeManager) Create(namespace string, jobName string, taskName string, volume spec.VolumeSpec) (*runtime.Mount, error) {
	if volume.HostVolume != "" {
		hostPath, ok := vm.hostVolumes[volume.HostVolume]
		if !ok {
			return nil, fmt.Errorf("host volume %q is not configured", volume.HostVolume)
		}
		info, err := os.Stat(hostPath)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("host volume %q is not available", volume.HostVolume)
		}
		return &runtime.Mount{HostPath: hostPath, ContainerPath: volume.Path, ReadOnly: volume.ReadOnly}, nil
	}
	hostPath, err := vm.getHostPath(namespace, jobName, taskName, volume.Name)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(hostPath, 0o750)
	if err != nil {
		return nil, fmt.Errorf("creating volume dir %s: %w", hostPath, err)
	}

	return &runtime.Mount{
		HostPath:      hostPath,
		ContainerPath: volume.Path,
		ReadOnly:      volume.ReadOnly,
	}, nil
}

// Check reports whether an allocation volume is available.
func (vm *VolumeManager) Check(namespace string, jobName string, taskName string, volume spec.VolumeSpec) (bool, error) {
	hostPath, err := vm.getHostPath(namespace, jobName, taskName, volume.Name)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(hostPath)
	if err != nil {
		return false, fmt.Errorf("checking volume dir %s: %w", hostPath, err)
	}

	return info.IsDir(), nil
}

// Delete removes an allocation-managed volume.
func (vm *VolumeManager) Delete(namespace string, jobName string, taskName string, volume spec.VolumeSpec) error {
	hostPath, err := vm.getHostPath(namespace, jobName, taskName, volume.Name)
	if err != nil {
		return err
	}

	err = os.RemoveAll(hostPath)
	if err != nil {
		return fmt.Errorf("deleting volume dir %s: %w", hostPath, err)
	}

	return nil
}

func (vm *VolumeManager) getHostPath(namespace string, jobName string, taskName string, volumeName string) (string, error) {
	parts := []string{namespace, jobName, taskName, volumeName}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || filepath.Base(part) != part || strings.ContainsAny(part, `/\\`) {
			return "", fmt.Errorf("invalid volume path component %q", part)
		}
	}
	return filepath.Join(vm.dataRootPath, "namespaces", namespace, jobName, taskName, volumeName), nil
}
