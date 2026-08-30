package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
)

type VolumeManager struct {
	dataRootPath string
}

func NewVolumeManager(dataRoot ...string) *VolumeManager {
	root := "/var/lib/trellis/data"
	if len(dataRoot) > 0 && dataRoot[0] != "" {
		root = dataRoot[0]
	}
	return &VolumeManager{
		dataRootPath: root,
	}
}

func (vm *VolumeManager) Create(namespace string, jobName string, taskName string, volume spec.VolumeSpec) (*runtime.Mount, error) {
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
	}, nil
}

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
