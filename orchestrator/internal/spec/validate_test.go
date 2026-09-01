package spec

import (
	"testing"
	"time"
)

func TestParseYAML(t *testing.T) {
	raw := []byte("namespace: default\nname: web\ntask_groups:\n  - name: api\n    count: 1\n    tasks:\n      - name: server\n        image: example/server:1\n        networking:\n          mode: host\n          ports:\n            - host_port: 8080\n              container_port: 80\n")
	job, err := ParseYAML(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if err := Validate(job); err != nil {
		t.Fatalf("validate parsed manifest: %v", err)
	}
	port := job.TaskGroups[0].Tasks[0].Networking.Ports[0]
	if port.HostPort != 8080 || port.ContainerPort != 80 {
		t.Fatalf("unexpected port mapping: %#v", port)
	}
}

func TestParseYAMLRejectsUnknownFields(t *testing.T) {
	raw := []byte("namespace: default\nname: web\nnetwork:\n  wireguard: true\ntask_groups:\n  - name: api\n    count: 1\n    network_mode: host\n    tasks:\n      - name: server\n        image: example/server:1\n        ports:\n          - host_port: 8080\n            container_port: 80\n")
	if _, err := ParseYAML(raw); err == nil {
		t.Fatal("expected unknown manifest fields to be rejected")
	}
}

func TestValidate(t *testing.T) {
	valid := &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server", Image: "example/server:1"}}}}}
	if err := Validate(valid); err != nil {
		t.Fatalf("valid job rejected: %v", err)
	}

	withExtensions := &JobSpec{Namespace: "default", Name: "proxy", TaskGroups: []TaskGroupSpec{{
		Name: "proxy", Count: 1, APIAccess: true,
		Labels: map[string]string{"trellis.expose": "true", "trellis/domain": "example.com"},
		Tasks:  []TaskSpec{{Name: "nginx", Image: "nginx:latest", Networking: &TaskNetworkingSpec{Mode: TaskNetworkHost}}},
	}}}
	if err := Validate(withExtensions); err != nil {
		t.Fatalf("valid job with extensions rejected: %v", err)
	}

	tests := []struct {
		name string
		job  *JobSpec
	}{
		{"nil", nil},
		{"missing name", &JobSpec{}},
		{"zero replicas", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Tasks: []TaskSpec{{Name: "server", Image: "image"}}}}}},
		{"missing image", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server"}}}}}},
		{"invalid port", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server", Image: "image", Networking: &TaskNetworkingSpec{Mode: TaskNetworkHost, Ports: []PortSpec{{ContainerPort: 70000}}}}}}}}},
		{"invalid network_mode", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server", Image: "image", Networking: &TaskNetworkingSpec{Mode: "bridge"}}}}}}},
		{"invalid label key", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Labels: map[string]string{"123bad": "v"}, Tasks: []TaskSpec{{Name: "server", Image: "image"}}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(test.job); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseConfigurableHealthAndRestartPolicy(t *testing.T) {
	raw := []byte("namespace: default\nname: web\ntask_groups:\n  - name: api\n    count: 1\n    restart:\n      max_restarts: 5\n      window: 2m\n    tasks:\n      - name: server\n        image: example/server:1\n        health_check:\n          type: tcp\n          port: 8080\n          interval: 15s\n          timeout: 3s\n          threshold: 2\n")
	job, err := ParseYAML(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if err := Validate(job); err != nil {
		t.Fatalf("validate manifest: %v", err)
	}
	restart := job.TaskGroups[0].Restart
	if restart == nil || restart.MaxRestarts != 5 || restart.Window != 2*time.Minute {
		t.Fatalf("unexpected restart policy: %#v", restart)
	}
	health := job.TaskGroups[0].Tasks[0].HealthCheck
	if health.Interval != 15*time.Second || health.Timeout != 3*time.Second || health.Threshold != 2 {
		t.Fatalf("unexpected health check timing: %#v", health)
	}
}

func TestValidateHealthAndRestartPolicy(t *testing.T) {
	job := func() *JobSpec {
		return &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{
			Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server", Image: "image", HealthCheck: &HealthCheckSpec{Type: "tcp", Port: 8080}}},
		}}}
	}

	tests := []struct {
		name   string
		mutate func(*JobSpec)
	}{
		{"negative health interval", func(j *JobSpec) { j.TaskGroups[0].Tasks[0].HealthCheck.Interval = -time.Second }},
		{"negative health timeout", func(j *JobSpec) { j.TaskGroups[0].Tasks[0].HealthCheck.Timeout = -time.Second }},
		{"negative health threshold", func(j *JobSpec) { j.TaskGroups[0].Tasks[0].HealthCheck.Threshold = -1 }},
		{"negative max restarts", func(j *JobSpec) { j.TaskGroups[0].Restart = &RestartPolicySpec{MaxRestarts: -1, Window: time.Minute} }},
		{"zero restart window", func(j *JobSpec) { j.TaskGroups[0].Restart = &RestartPolicySpec{MaxRestarts: 0} }},
		{"negative restart window", func(j *JobSpec) { j.TaskGroups[0].Restart = &RestartPolicySpec{MaxRestarts: 1, Window: -time.Minute} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := job()
			test.mutate(candidate)
			if err := Validate(candidate); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	valid := job()
	valid.TaskGroups[0].Restart = &RestartPolicySpec{MaxRestarts: 0, Window: time.Minute}
	if err := Validate(valid); err != nil {
		t.Fatalf("zero-restart policy rejected: %v", err)
	}
}

func TestParseConstraints(t *testing.T) {
	raw := []byte("namespace: default\nname: web\ntask_groups:\n  - name: api\n    count: 1\n    constraints:\n      - attribute: os\n        value: linux\n      - attribute: arch\n        value: arm64\n    tasks:\n      - name: server\n        image: example/server:1\n")
	job, err := ParseYAML(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if err := Validate(job); err != nil {
		t.Fatalf("validate manifest: %v", err)
	}
	constraints := job.TaskGroups[0].Constraints
	if len(constraints) != 2 || constraints[0].Attribute != "os" || constraints[0].Value != "linux" || constraints[1].Attribute != "arch" || constraints[1].Value != "arm64" {
		t.Fatalf("unexpected constraints: %#v", constraints)
	}
}

func TestValidateConstraints(t *testing.T) {
	job := func(constraints ...ConstraintSpec) *JobSpec {
		return &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{
			Name: "api", Count: 1, Constraints: constraints, Tasks: []TaskSpec{{Name: "server", Image: "image"}},
		}}}
	}

	valid := job(
		ConstraintSpec{Attribute: "os", Value: "linux"},
		ConstraintSpec{Attribute: "example.com/accelerator", Value: "gpu"},
	)
	if err := Validate(valid); err != nil {
		t.Fatalf("valid constraints rejected: %v", err)
	}

	tests := []struct {
		name        string
		constraints []ConstraintSpec
	}{
		{"missing attribute", []ConstraintSpec{{Value: "linux"}}},
		{"invalid attribute", []ConstraintSpec{{Attribute: "bad attribute", Value: "linux"}}},
		{"missing value", []ConstraintSpec{{Attribute: "os"}}},
		{"duplicate attribute", []ConstraintSpec{{Attribute: "arch", Value: "amd64"}, {Attribute: "arch", Value: "arm64"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(job(test.constraints...)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
