package spec

import (
	"testing"
)

func TestParseAPIAccessModes(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  APIAccessMode
	}{
		{name: "namespace", value: "namespace", want: APIAccessNamespace},
		{name: "cluster", value: "cluster", want: APIAccessCluster},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := []byte("namespace: default\nname: api-client\ntask_groups:\n  - name: client\n    count: 1\n    api_access: " + test.value + "\n    tasks:\n      - name: client\n        image: example/client:1\n")
			job, err := ParseYAML(raw)
			if err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
			if got := job.TaskGroups[0].APIAccess; got != test.want {
				t.Fatalf("api_access = %q, want %q", got, test.want)
			}
			if err := Validate(job); err != nil {
				t.Fatalf("validate manifest: %v", err)
			}
		})
	}
}

func TestValidateRejectsInvalidAPIAccessMode(t *testing.T) {
	job := &JobSpec{Namespace: "default", Name: "api-client", TaskGroups: []TaskGroupSpec{{
		Name: "client", Count: 1, APIAccess: "other", Tasks: []TaskSpec{{Name: "client", Image: "example/client:1"}},
	}}}
	if err := Validate(job); err == nil {
		t.Fatal("expected invalid api_access mode to be rejected")
	}
}
