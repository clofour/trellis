package agent

import (
	"testing"

	"github.com/clofour/trellis/internal/spec"
)

func TestVolumeManagerRejectsTraversal(t *testing.T) {
	manager := NewVolumeManager(t.TempDir())
	invalid := []struct{ job, task, volume string }{{"../job", "task", "data"}, {"job", "../task", "data"}, {"job", "task", "../data"}}
	for _, tc := range invalid {
		if _, err := manager.Create(tc.job, tc.task, spec.VolumeSpec{Name: tc.volume, Path: "/data"}); err == nil {
			t.Errorf("Create(%q, %q, %q) succeeded", tc.job, tc.task, tc.volume)
		}
	}
}
