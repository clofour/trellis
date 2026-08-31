package agent

import (
	"os"
	"testing"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/spec"
)

func TestPrepareSecretsDeliversEnvAndMemoryBackedFile(t *testing.T) {
	delivered := []api.DeliveredSecret{
		{Task: "api", Name: "password", Target: spec.SecretTargetEnv, Env: "PASSWORD", Value: []byte("env-value")},
		{Task: "api", Name: "key", Target: spec.SecretTargetFile, Path: "/run/trellis-secrets/key", Mode: 0o400, Value: []byte("file-value")},
		{Task: "other", Name: "ignored", Target: spec.SecretTargetEnv, Env: "IGNORED", Value: []byte("ignored")},
	}
	dir, env, mounts, err := prepareSecrets("alloc", "api", delivered)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if env["PASSWORD"] != "env-value" || env["IGNORED"] != "" {
		t.Fatalf("unexpected env: %#v", env)
	}
	if len(mounts) != 1 || !mounts[0].ReadOnly || mounts[0].ContainerPath != "/run/trellis-secrets/key" {
		t.Fatalf("unexpected mounts: %#v", mounts)
	}
	value, err := os.ReadFile(mounts[0].HostPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "file-value" {
		t.Fatalf("got %q", value)
	}
	info, err := os.Stat(mounts[0].HostPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o400 {
		t.Fatalf("mode is %o", info.Mode().Perm())
	}
}
