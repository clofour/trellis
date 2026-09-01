package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadConfigPreservesTLSFlags(t *testing.T) {
	previousConfig := config
	t.Cleanup(func() { config = previousConfig })

	t.Setenv("TRELLIS_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	config = CLIConfig{}
	root := testRootCommand()
	flags := root.PersistentFlags()
	if err := flags.Parse([]string{
		"--ca-cert", "cluster-ca.pem",
		"--cert", "client.pem",
		"--key", "client-key.pem",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := loadConfig(root); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.CACert != "cluster-ca.pem" {
		t.Fatalf("CA certificate flag was not preserved: got %q", config.CACert)
	}
	if config.Cert != "client.pem" {
		t.Fatalf("client certificate flag was not preserved: got %q", config.Cert)
	}
	if config.Key != "client-key.pem" {
		t.Fatalf("client key flag was not preserved: got %q", config.Key)
	}
}

func TestLoadConfigUsesNamedContextThenExplicitFlags(t *testing.T) {
	previousConfig := config
	t.Cleanup(func() { config = previousConfig })

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`current_context: production
contexts:
  production:
    server_addr: prod.example:8128
    cluster_token: prod-token
    namespace: payments
    ca_cert: prod-ca
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRELLIS_CONFIG", path)
	config = CLIConfig{}
	root := testRootCommand()
	if err := root.PersistentFlags().Parse([]string{"--server-addr", "override.example:8128"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := loadConfig(root); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.Context != "production" {
		t.Fatalf("context = %q, want production", config.Context)
	}
	if config.ServerAddr != "override.example:8128" {
		t.Fatalf("server = %q", config.ServerAddr)
	}
	if config.ClusterToken != "prod-token" || config.Namespace != "payments" {
		t.Fatalf("context values not loaded: %#v", config)
	}
	if config.CACertPEM != "prod-ca" {
		t.Fatalf("CA = %q", config.CACertPEM)
	}
}

func TestWriteUserConfigProtectsTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	file := fileConfig{CurrentContext: "prod", Contexts: map[string]contextFileConfig{
		"prod": {ServerAddr: "prod:8128", ClusterToken: "secret", Namespace: "default"},
	}}
	if err := writeUserConfig(path, file); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
	loaded, err := readUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Contexts["prod"].ClusterToken != "secret" {
		t.Fatal("saved context did not round-trip")
	}
}

func testRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "trellis"}
	flags := root.PersistentFlags()
	flags.StringVar(&config.Context, "context", "", "")
	flags.StringVar(&config.ServerAddr, "server-addr", "localhost:8128", "")
	flags.StringVar(&config.ClusterToken, "cluster-token", "", "")
	flags.StringVar(&config.Namespace, "namespace", "", "")
	flags.StringVar(&config.CACert, "ca-cert", "", "")
	flags.StringVar(&config.Cert, "cert", "", "")
	flags.StringVar(&config.Key, "key", "", "")
	flags.StringVarP(&config.Output, "output", "o", "table", "")
	return root
}
