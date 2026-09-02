package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestLoadNodeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trellis.yaml")
	if err := os.WriteFile(path, []byte(`cluster: production
bootstrap_token: trls_boot_test
agent_advertise: node-a:8127
wireguard_port: 51900
labels:
  - storage=fast
host_volumes:
  - data=/srv/data
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config{Cluster: "default", WireGuardPort: 51820}
	if err := loadNodeConfig(path, cfg, pflag.NewFlagSet("test", pflag.ContinueOnError)); err != nil {
		t.Fatal(err)
	}
	if cfg.Cluster != "production" || cfg.ClusterToken != "trls_boot_test" || cfg.AgentAdvertise != "node-a:8127" || cfg.WireGuardPort != 51900 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if len(cfg.Labels) != 1 || cfg.Labels[0] != "storage=fast" || len(cfg.HostVolumes) != 1 || cfg.HostVolumes[0] != "data=/srv/data" {
		t.Fatalf("unexpected lists: labels=%v volumes=%v", cfg.Labels, cfg.HostVolumes)
	}
}

func TestLoadNodeConfigFlagsOverrideFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trellis.yaml")
	if err := os.WriteFile(path, []byte("cluster: from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("cluster", "", "")
	if err := flags.Set("cluster", "from-flag"); err != nil {
		t.Fatal(err)
	}
	cfg := &config{Cluster: "from-flag"}
	if err := loadNodeConfig(path, cfg, flags); err != nil {
		t.Fatal(err)
	}
	if cfg.Cluster != "from-flag" {
		t.Fatalf("file overrode explicit flag: %q", cfg.Cluster)
	}
}

func TestLoadNodeConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trellis.yaml")
	if err := os.WriteFile(path, []byte("bootstrap_token: token\nclustr: typo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadNodeConfig(path, &config{}, pflag.NewFlagSet("test", pflag.ContinueOnError)); err == nil {
		t.Fatal("expected unknown config field to be rejected")
	}
}
