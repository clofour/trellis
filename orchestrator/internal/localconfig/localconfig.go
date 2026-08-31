// Package localconfig manages the runtime connection file that a local
// trellis-node writes on startup. The CLI reads it for zero-config local
// access; machines without a node fall back to the user config file or flags.
package localconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultPath is where trellis-node writes its connection file. /run is
// tmpfs on modern Linux, so the file is automatically absent when no node
// is running and cleaned up on reboot.
const DefaultPath = "/run/trellis/local.yaml"

// Config is the connection information the CLI needs to reach a cluster.
type Config struct {
	ServerAddr   string `yaml:"server_addr"`
	ClusterToken string `yaml:"cluster_token"`
	CACert       string `yaml:"ca_cert,omitempty"` // inline PEM
}

// Write atomically writes cfg to path, creating parent directories as needed.
func Write(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install: %w", err)
	}
	return nil
}

// Read parses the config file at path. Returns an error wrapping
// os.ErrNotExist when the file is absent.
func Read(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &cfg, nil
}
