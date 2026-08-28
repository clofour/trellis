package spec

import (
	"errors"
	"fmt"
	"strings"
)

func Validate(spec *JobSpec) error {
	if spec == nil {
		return errors.New("job is required")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return errors.New("job name is required")
	}
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
	return nil
}
