package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
var labelKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._/-]{0,62}$`)

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
	if strings.TrimSpace(spec.Namespace) == "" {
		return errors.New("job namespace is required")
	}
	if !identifierPattern.MatchString(spec.Namespace) {
		return errors.New("job namespace must be a safe identifier")
	}
	if len(spec.TaskGroups) == 0 {
		return errors.New("at least one task group is required")
	}

	groups := make(map[string]struct{})
	for i, group := range spec.TaskGroups {
		if strings.TrimSpace(group.Name) == "" {
			return fmt.Errorf("task group %d: name is required", i)
		}
		if !identifierPattern.MatchString(group.Name) {
			return fmt.Errorf("task group %q: name must be a safe identifier", group.Name)
		}
		if _, exists := groups[group.Name]; exists {
			return fmt.Errorf("duplicate task group %q", group.Name)
		}
		groups[group.Name] = struct{}{}
		if !group.Runtime.Valid() {
			return fmt.Errorf("task group %q: unsupported runtime %q", group.Name, group.Runtime)
		}
		if !group.NetworkMode.Valid() {
			return fmt.Errorf("task group %q: unsupported network_mode %q", group.Name, group.NetworkMode)
		}
		if group.Restart != nil {
			if group.Restart.MaxRestarts < 0 {
				return fmt.Errorf("task group %q: restart max_restarts must be at least 0", group.Name)
			}
			if group.Restart.Window <= 0 {
				return fmt.Errorf("task group %q: restart window must be positive", group.Name)
			}
		}

		constraints := make(map[string]struct{}, len(group.Constraints))
		for _, constraint := range group.Constraints {
			if !labelKeyPattern.MatchString(constraint.Attribute) {
				return fmt.Errorf("task group %q: invalid constraint attribute %q", group.Name, constraint.Attribute)
			}
			if strings.TrimSpace(constraint.Value) == "" {
				return fmt.Errorf("task group %q: constraint %q requires a value", group.Name, constraint.Attribute)
			}
			if _, exists := constraints[constraint.Attribute]; exists {
				return fmt.Errorf("task group %q: duplicate constraint attribute %q", group.Name, constraint.Attribute)
			}
			constraints[constraint.Attribute] = struct{}{}
		}
		for key, value := range group.Labels {
			if !labelKeyPattern.MatchString(key) {
				return fmt.Errorf("task group %q: invalid label key %q", group.Name, key)
			}
			if len(value) > 256 {
				return fmt.Errorf("task group %q: label value for %q exceeds 256 characters", group.Name, key)
			}
		}
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
			if !identifierPattern.MatchString(task.Name) {
				return fmt.Errorf("task group %q task %q: name must be a safe identifier", group.Name, task.Name)
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

			volumes := make(map[string]struct{})
			for _, volume := range task.Volumes {
				if !identifierPattern.MatchString(volume.Name) || strings.TrimSpace(volume.Path) == "" || !strings.HasPrefix(volume.Path, "/") {
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
				if task.HealthCheck.Interval != 0 && task.HealthCheck.Interval <= 0 {
					return fmt.Errorf("task group %q task %q: health check interval must be positive", group.Name, task.Name)
				}
				if task.HealthCheck.Timeout != 0 && task.HealthCheck.Timeout <= 0 {
					return fmt.Errorf("task group %q task %q: health check timeout must be positive", group.Name, task.Name)
				}
				if task.HealthCheck.Threshold != 0 && task.HealthCheck.Threshold < 1 {
					return fmt.Errorf("task group %q task %q: health check threshold must be at least 1", group.Name, task.Name)
				}
				switch task.HealthCheck.Type {
				case HealthCheckHTTP, HealthCheckTCP:
					if task.HealthCheck.Port < 1 || task.HealthCheck.Port > 65535 {
						return fmt.Errorf("task group %q task %q: health check port is required", group.Name, task.Name)
					}
				case HealthCheckScript:
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
