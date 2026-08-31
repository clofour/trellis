package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadConfigPreservesTLSFlags(t *testing.T) {
	previousConfig := config
	t.Cleanup(func() {
		config = previousConfig
	})

	t.Setenv("TRELLIS_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	config = CLIConfig{}

	root := &cobra.Command{Use: "trellis"}
	flags := root.PersistentFlags()
	flags.StringVar(&config.ServerAddr, "server-addr", "localhost:8128", "")
	flags.StringVar(&config.ClusterToken, "cluster-token", "", "")
	flags.StringVar(&config.Namespace, "namespace", "", "")
	flags.StringVar(&config.CACert, "ca-cert", "", "")
	flags.StringVar(&config.Cert, "cert", "", "")
	flags.StringVar(&config.Key, "key", "", "")
	flags.StringVarP(&config.Output, "output", "o", "table", "")

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
