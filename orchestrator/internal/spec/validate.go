package spec

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
var labelKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._/-]{0,62}$`)
var envPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidIdentifier(value string) bool { return identifierPattern.MatchString(value) }

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
	var totalCPU, totalMemory int
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
		if group.Update != nil && group.Update.MaxUnavailable < 1 {
			return fmt.Errorf("task group %q: update max_unavailable must be at least 1", group.Name)
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
		for k, v := range group.Labels {
			if !labelKeyPattern.MatchString(k) {
				return fmt.Errorf("task group %q: invalid label key %q", group.Name, k)
			}
			if len(v) > 256 {
				return fmt.Errorf("task group %q: label value for %q exceeds 256 characters", group.Name, k)
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
			secretNames, secretEnvs, secretPaths := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
			for _, secret := range task.Secrets {
				if !ValidIdentifier(secret.Name) {
					return fmt.Errorf("task group %q task %q: invalid secret name %q", group.Name, task.Name, secret.Name)
				}
				if _, ok := secretNames[secret.Name]; ok {
					return fmt.Errorf("task group %q task %q: duplicate secret %q", group.Name, task.Name, secret.Name)
				}
				secretNames[secret.Name] = struct{}{}
				switch secret.Target {
				case SecretTargetEnv:
					if !envPattern.MatchString(secret.Env) || secret.Path != "" || secret.Mode != 0 {
						return fmt.Errorf("task group %q task %q: env secret %q requires only a valid env name", group.Name, task.Name, secret.Name)
					}
					if _, ok := secretEnvs[secret.Env]; ok {
						return fmt.Errorf("task group %q task %q: duplicate secret env %q", group.Name, task.Name, secret.Env)
					}
					if _, ok := task.Env[secret.Env]; ok {
						return fmt.Errorf("task group %q task %q: secret env %q conflicts with env", group.Name, task.Name, secret.Env)
					}
					secretEnvs[secret.Env] = struct{}{}
				case SecretTargetFile:
					clean := filepath.Clean(secret.Path)
					if secret.Env != "" || clean != secret.Path || !strings.HasPrefix(clean, "/run/trellis-secrets/") {
						return fmt.Errorf("task group %q task %q: file secret %q requires a clean path below /run/trellis-secrets", group.Name, task.Name, secret.Name)
					}
					if secret.Mode != 0 && secret.Mode != 0o400 && secret.Mode != 0o600 {
						return fmt.Errorf("task group %q task %q: file secret %q mode must be 0400 or 0600", group.Name, task.Name, secret.Name)
					}
					if _, ok := secretPaths[clean]; ok {
						return fmt.Errorf("task group %q task %q: duplicate secret path %q", group.Name, task.Name, clean)
					}
					secretPaths[clean] = struct{}{}
				default:
					return fmt.Errorf("task group %q task %q: secret %q target must be env or file", group.Name, task.Name, secret.Name)
				}
			}
			if strings.TrimSpace(task.Image) == "" {
				return fmt.Errorf("task group %q task %q: image is required", group.Name, task.Name)
			}
			if task.Resources != nil && (task.Resources.CPU < 0 || task.Resources.Memory < 0) {
				return fmt.Errorf("task group %q task %q: resources cannot be negative", group.Name, task.Name)
			}
			if task.Resources != nil {
				totalCPU += task.Resources.CPU * group.Count
				totalMemory += task.Resources.Memory * group.Count
			}
			volumes := make(map[string]struct{})
			for _, volume := range task.Volumes {
				if !identifierPattern.MatchString(volume.Name) || strings.TrimSpace(volume.Path) == "" || !strings.HasPrefix(volume.Path, "/") {
					return fmt.Errorf("task group %q task %q: volume name and absolute path are required", group.Name, task.Name)
				}
				if volume.HostVolume != "" && !identifierPattern.MatchString(volume.HostVolume) {
					return fmt.Errorf("task group %q task %q: invalid host volume %q", group.Name, task.Name, volume.HostVolume)
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
	_ = totalCPU
	_ = totalMemory
	return nil
}
