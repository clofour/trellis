package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clofour/trellis/internal/models"
)

type VolumeManager struct {
	dataRootPath string
}

func NewVolumeManager() *VolumeManager {
	return &VolumeManager{
		dataRootPath: "/var/lib/trellis/data",
	}
}

func (vm *VolumeManager) Create(jobName string, taskName string, volume models.Volume) error {
	hostPath := vm.getHostPath(jobName, taskName, volume.Name)

	err := os.MkdirAll(hostPath, 0755)
	if err != nil {
		return fmt.Errorf("creating volume dir %s: %w", hostPath, err)
	}

	return nil
}

func (vm *VolumeManager) Check(jobName string, taskName string, volume models.Volume) (bool, error) {
	hostPath := vm.getHostPath(jobName, taskName, volume.Name)

	info, err := os.Stat(hostPath)
	if err != nil {
		return false, fmt.Errorf("checking volume dir %s: %w", hostPath, err)
	}

	return info.IsDir(), nil
}

func (vm *VolumeManager) Delete(jobName string, taskName string, volume models.Volume) error {
	hostPath := vm.getHostPath(jobName, taskName, volume.Name)

	err := os.RemoveAll(hostPath)
	if err != nil {
		return fmt.Errorf("deleting volume dir %s: %w", hostPath, err)
	}

	return nil
}

func (vm *VolumeManager) getHostPath(jobName string, taskName string, volumeName string) string {
	return filepath.Join(vm.dataRootPath, jobName, taskName, volumeName)
}
