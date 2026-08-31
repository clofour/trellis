package spec

import "testing"

func TestValidateNamespaceNetworkAndGroupRuntime(t *testing.T) {
	job := &JobSpec{
		Namespace: "acme",
		Name:      "web",
		TaskGroups: []TaskGroupSpec{{
			Name: "web", Count: 2, Runtime: "runsc",
			Tasks: []TaskSpec{{Name: "app", Image: "image", Resources: &ResourcesSpec{CPU: 500, Memory: 512}, Networking: &TaskNetworkingSpec{Mode: TaskNetworkWireGuard}}},
		}},
	}
	if err := Validate(job); err != nil {
		t.Fatalf("expected valid namespaced job: %v", err)
	}
	job.TaskGroups[0].Runtime = "other"
	if err := Validate(job); err == nil {
		t.Fatal("expected unsupported group runtime to fail")
	}
}

func TestValidateRequiresSafeNamespace(t *testing.T) {
	job := &JobSpec{Namespace: "../other", Name: "web", TaskGroups: []TaskGroupSpec{{Name: "g", Count: 1, Tasks: []TaskSpec{{Name: "t", Image: "image"}}}}}
	if err := Validate(job); err == nil {
		t.Fatal("expected unsafe namespace to fail")
	}
}
