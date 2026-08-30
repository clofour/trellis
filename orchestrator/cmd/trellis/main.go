package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type CLIConfig struct {
	ServerAddr   string
	ClusterToken string
	Namespace    string
	CACert       string
	Cert         string
	Key          string
	Output       string
}

var config CLIConfig

type fileConfig struct {
	ServerAddr   *string `yaml:"server_addr"`
	ClusterToken *string `yaml:"cluster_token"`
	Namespace    *string `yaml:"namespace"`
}

func main() {
	root := &cobra.Command{
		Use:   "trellis",
		Short: "Trellis CLI",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return loadConfig(cmd)
		},
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&config.ServerAddr, "server-addr", "localhost:8128", "Server HTTP API address")
	persistentFlags.StringVar(&config.ClusterToken, "cluster-token", "", "Cluster token")
	persistentFlags.StringVar(&config.Namespace, "namespace", "", "Namespace scope for job queries")
	persistentFlags.StringVar(&config.CACert, "ca-cert", "", "Path to cluster CA certificate (PEM)")
	persistentFlags.StringVar(&config.Cert, "cert", "", "Path to client certificate (PEM)")
	persistentFlags.StringVar(&config.Key, "key", "", "Path to client private key (PEM)")
	persistentFlags.StringVarP(&config.Output, "output", "o", "table", "Output format (table or json)")

	root.AddCommand(NewJobsCmd())
	root.AddCommand(NewNodesCmd())

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func buildCLITLSConfig() (*tls.Config, error) {
	if config.CACert == "" {
		return nil, nil
	}
	caPEM, err := os.ReadFile(config.CACert)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}
	cfg := &tls.Config{
		RootCAs:    pool,
		ServerName: "trellis-node",
	}
	if config.Cert != "" && config.Key != "" {
		cert, err := tls.LoadX509KeyPair(config.Cert, config.Key)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

func loadConfig(cmd *cobra.Command) error {
	flagConfig := config
	merged := CLIConfig{
		ServerAddr: "localhost:8128",
		Output:     flagConfig.Output,
	}

	path := os.Getenv("TRELLIS_CONFIG")
	if path == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("determine config directory: %w", err)
		}
		path = filepath.Join(configDir, "trellis", "config.yaml")
	}

	content, err := os.ReadFile(path)
	if err == nil {
		var file fileConfig
		if err := yaml.Unmarshal(content, &file); err != nil {
			return fmt.Errorf("parse config file %s: %w", path, err)
		}
		if file.ServerAddr != nil {
			merged.ServerAddr = *file.ServerAddr
		}
		if file.ClusterToken != nil {
			merged.ClusterToken = *file.ClusterToken
		}
		if file.Namespace != nil {
			merged.Namespace = *file.Namespace
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config file %s: %w", path, err)
	}

	if value, ok := os.LookupEnv("TRELLIS_ADDR"); ok {
		merged.ServerAddr = value
	}
	if value, ok := os.LookupEnv("TRELLIS_TOKEN"); ok {
		merged.ClusterToken = value
	}
	if value, ok := os.LookupEnv("TRELLIS_NAMESPACE"); ok {
		merged.Namespace = value
	}

	flags := cmd.Root().PersistentFlags()
	if flags.Changed("server-addr") {
		merged.ServerAddr = flagConfig.ServerAddr
	}
	if flags.Changed("cluster-token") {
		merged.ClusterToken = flagConfig.ClusterToken
	}
	if flags.Changed("namespace") {
		merged.Namespace = flagConfig.Namespace
	}

	if merged.Output != "table" && merged.Output != "json" {
		return fmt.Errorf("invalid output format %q: must be table or json", merged.Output)
	}

	config = merged
	return nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
