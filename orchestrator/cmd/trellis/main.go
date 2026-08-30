package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type CLIConfig struct {
	ServerAddr   string
	ClusterToken string
	Namespace    string
	CACert       string
	Cert         string
	Key          string
}

var config CLIConfig

func main() {
	root := &cobra.Command{
		Use:   "trellis",
		Short: "Trellis CLI",
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&config.ServerAddr, "server-addr", "localhost:8128", "Server HTTP API address")
	persistentFlags.StringVar(&config.ClusterToken, "cluster-token", "", "Cluster token")
	persistentFlags.StringVar(&config.Namespace, "namespace", "", "Namespace scope for job queries")
	persistentFlags.StringVar(&config.CACert, "ca-cert", "", "Path to cluster CA certificate (PEM)")
	persistentFlags.StringVar(&config.Cert, "cert", "", "Path to client certificate (PEM)")
	persistentFlags.StringVar(&config.Key, "key", "", "Path to client private key (PEM)")

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
