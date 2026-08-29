package spec

import "testing"

func TestValidateIsolatedJob(t *testing.T) {
	job := &JobSpec{Name: "web", Tenant: "acme", Isolation: &IsolationSpec{
		Runtime: "runsc", Network: &WireGuardSpec{Enabled: true, Network: "acme"},
		Quota: &ResourcesSpec{CPU: 1000, Memory: 1024},
	}, TaskGroups: []TaskGroupSpec{{Name: "web", Count: 2, Tasks: []TaskSpec{{
		Name: "server", Image: "example/web", Resources: &ResourcesSpec{CPU: 500, Memory: 512},
	}}}}}
	if err := Validate(job); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	job.TaskGroups[0].Count = 3
	if err := Validate(job); err == nil {
		t.Fatal("Validate() accepted a job exceeding its quota")
	}
}

func TestValidateIsolationRequiresSafeTenantAndRunsc(t *testing.T) {
	job := &JobSpec{Name: "web", Tenant: "../other", Isolation: &IsolationSpec{Runtime: "runc"}, TaskGroups: []TaskGroupSpec{{Name: "g", Count: 1, Tasks: []TaskSpec{{Name: "t", Image: "image"}}}}}
	if err := Validate(job); err == nil {
		t.Fatal("Validate() accepted an unsafe tenant")
	}
}
