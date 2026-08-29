package spec

import "testing"

func TestParseYAML(t *testing.T) {
	raw := []byte("namespace: default\nname: web\ntask_groups:\n  - name: api\n    count: 1\n    tasks:\n      - name: server\n        image: example/server:1\n        ports:\n          - host_port: 8080\n            container_port: 80\n")
	job, err := ParseYAML(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if err := Validate(job); err != nil {
		t.Fatalf("validate parsed manifest: %v", err)
	}
	port := job.TaskGroups[0].Tasks[0].Ports[0]
	if port.HostPort != 8080 || port.ContainerPort != 80 {
		t.Fatalf("unexpected port mapping: %#v", port)
	}
}

func TestValidate(t *testing.T) {
	valid := &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server", Image: "example/server:1"}}}}}
	if err := Validate(valid); err != nil {
		t.Fatalf("valid job rejected: %v", err)
	}

	tests := []struct {
		name string
		job  *JobSpec
	}{
		{"nil", nil},
		{"missing name", &JobSpec{}},
		{"zero replicas", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Tasks: []TaskSpec{{Name: "server", Image: "image"}}}}}},
		{"missing image", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server"}}}}}},
		{"invalid port", &JobSpec{Namespace: "default", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "api", Count: 1, Tasks: []TaskSpec{{Name: "server", Image: "image", Ports: []PortSpec{{ContainerPort: 70000}}}}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(test.job); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
