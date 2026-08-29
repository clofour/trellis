package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)

func Validate(spec *JobSpec) error {
	if spec == nil {
		return errors.New("job is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return errors.New("job name is required")
	}
	if !identifierPattern.MatchString(spec.Name) {
		return errors.New("job name must be a safe identifier")
	}
	if spec.Tenant != "" && !identifierPattern.MatchString(spec.Tenant) {
		return errors.New("tenant must be a safe identifier")
	}
	if spec.Isolation != nil {
		if strings.TrimSpace(spec.Tenant) == "" {
			return errors.New("tenant is required when isolation is enabled")
		}
		if spec.Isolation.Runtime != "runsc" {
			return fmt.Errorf("isolation runtime must be %q", "runsc")
		}
		if spec.Isolation.Network == nil || !spec.Isolation.Network.Enabled || strings.TrimSpace(spec.Isolation.Network.Network) == "" {
			return errors.New("an enabled WireGuard network is required when isolation is enabled")
		}
		if spec.Isolation.Quota == nil || spec.Isolation.Quota.CPU <= 0 || spec.Isolation.Quota.Memory <= 0 {
			return errors.New("positive CPU and memory quotas are required when isolation is enabled")
		}
	}
	var totalCPU, totalMemory int
	if len(spec.TaskGroups) == 0 {
		return errors.New("at least one task group is required")
	}
	groups := make(map[string]struct{})
	for i, group := range spec.TaskGroups {
		if strings.TrimSpace(group.Name) == "" {
			return fmt.Errorf("task group %d: name is required", i)
		}
		if _, exists := groups[group.Name]; exists {
			return fmt.Errorf("duplicate task group %q", group.Name)
		}
		groups[group.Name] = struct{}{}
		if group.Count < 1 {
			return fmt.Errorf("task group %q: count must be at least 1", group.Name)
		}
		if len(group.Tasks) == 0 {
			return fmt.Errorf("task group %q: at least one task is required", group.Name)
		}
		tasks := make(map[string]struct{})
		for j, task := range group.Tasks {
			if strings.TrimSpace(task.Name) == "" {
				return fmt.Errorf("task group %q task %d: name is required", group.Name, j)
			}
			if _, exists := tasks[task.Name]; exists {
				return fmt.Errorf("task group %q: duplicate task %q", group.Name, task.Name)
			}
			tasks[task.Name] = struct{}{}
			if strings.TrimSpace(task.Image) == "" {
				return fmt.Errorf("task group %q task %q: image is required", group.Name, task.Name)
			}
			if task.Resources != nil && (task.Resources.CPU < 0 || task.Resources.Memory < 0) {
				return fmt.Errorf("task group %q task %q: resources cannot be negative", group.Name, task.Name)
			}
			if spec.Isolation != nil && (task.Resources == nil || task.Resources.CPU <= 0 || task.Resources.Memory <= 0) {
				return fmt.Errorf("task group %q task %q: positive resource limits are required for isolated jobs", group.Name, task.Name)
			}
			if task.Resources != nil {
				totalCPU += task.Resources.CPU * group.Count
				totalMemory += task.Resources.Memory * group.Count
			}
			volumes := make(map[string]struct{})
			for _, volume := range task.Volumes {
				if strings.TrimSpace(volume.Name) == "" || strings.TrimSpace(volume.Path) == "" || !strings.HasPrefix(volume.Path, "/") {
					return fmt.Errorf("task group %q task %q: volume name and absolute path are required", group.Name, task.Name)
				}
				if _, exists := volumes[volume.Name]; exists {
					return fmt.Errorf("task group %q task %q: duplicate volume %q", group.Name, task.Name, volume.Name)
				}
				volumes[volume.Name] = struct{}{}
			}
			for _, port := range task.Ports {
				if port.HostPort < 0 || port.HostPort > 65535 || port.ContainerPort < 1 || port.ContainerPort > 65535 {
					return fmt.Errorf("task group %q task %q: invalid port mapping %d:%d", group.Name, task.Name, port.HostPort, port.ContainerPort)
				}
			}
			if task.HealthCheck != nil {
				switch task.HealthCheck.Type {
				case "http", "tcp":
					if task.HealthCheck.Port < 1 || task.HealthCheck.Port > 65535 {
						return fmt.Errorf("task group %q task %q: health check port is required", group.Name, task.Name)
					}
				case "script":
					if len(task.HealthCheck.Command) == 0 {
						return fmt.Errorf("task group %q task %q: health check command is required", group.Name, task.Name)
					}
				default:
					return fmt.Errorf("task group %q task %q: unsupported health check type %q", group.Name, task.Name, task.HealthCheck.Type)
				}
			}
		}
	}
	if spec.Isolation != nil && (totalCPU > spec.Isolation.Quota.CPU || totalMemory > spec.Isolation.Quota.Memory) {
		return fmt.Errorf("job requests cpu=%d memory=%d, exceeding tenant quota cpu=%d memory=%d", totalCPU, totalMemory, spec.Isolation.Quota.CPU, spec.Isolation.Quota.Memory)
	}
	return nil
}
