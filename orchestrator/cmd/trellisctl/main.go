package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/clofour/trellis/internal/localconfig"
	"github.com/clofour/trellis/internal/version"
	"github.com/spf13/cobra"
)

type CLIConfig struct {
	Context      string
	ServerAddr   string
	ClusterToken string
	Namespace    string
	CACert       string // path (from --ca-cert flag or TRELLIS_CA_CERT)
	CACertPEM    string // inline PEM (from run file or user config file)
	Cert         string
	Key          string
	Output       string
}

var config CLIConfig

type contextFileConfig struct {
	ServerAddr   string `yaml:"server_addr,omitempty"`
	ClusterToken string `yaml:"cluster_token,omitempty"`
	Namespace    string `yaml:"namespace,omitempty"`
	CACert       string `yaml:"ca_cert,omitempty"`
	Cert         string `yaml:"cert,omitempty"`
	Key          string `yaml:"key,omitempty"`
}

type fileConfig struct {
	CurrentContext string                       `yaml:"current_context,omitempty"`
	Contexts       map[string]contextFileConfig `yaml:"contexts,omitempty"`

	// Legacy flat fields remain supported as defaults beneath a selected context.
	ServerAddr   *string `yaml:"server_addr,omitempty"`
	ClusterToken *string `yaml:"cluster_token,omitempty"`
	Namespace    *string `yaml:"namespace,omitempty"`
	CACert       *string `yaml:"ca_cert,omitempty"`
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "trellisctl",
		Short:   "Operate Trellis clusters",
		Version: version.Current(),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return loadConfig(cmd)
		},
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&config.Context, "context", "", "Named cluster context to use for this command")
	persistentFlags.StringVar(&config.ServerAddr, "server-addr", "localhost:8128", "Cluster API address")
	persistentFlags.StringVar(&config.ClusterToken, "cluster-token", "", "Cluster token")
	persistentFlags.StringVar(&config.Namespace, "namespace", "", "Namespace scope for jobs, allocations, and secrets")
	persistentFlags.StringVar(&config.CACert, "ca-cert", "", "Path to cluster CA certificate (PEM)")
	persistentFlags.StringVar(&config.Cert, "cert", "", "Path to client certificate (PEM)")
	persistentFlags.StringVar(&config.Key, "key", "", "Path to client private key (PEM)")
	persistentFlags.StringVarP(&config.Output, "output", "o", "table", "Output format (table or json)")

	root.AddCommand(NewContextCmd())
	root.AddCommand(NewJobsCmd())
	root.AddCommand(NewNodesCmd())
	root.AddCommand(NewSecretsCmd())
	root.AddCommand(NewBackupCmd())
	root.AddCommand(NewVersionCmd())
	return root
}

func buildCLITLSConfig() (*tls.Config, error) {
	var caPEM []byte
	switch {
	case config.CACert != "":
		data, err := os.ReadFile(config.CACert)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %w", err)
		}
		caPEM = data
	case config.CACertPEM != "":
		caPEM = []byte(config.CACertPEM)
	default:
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}
	cfg := &tls.Config{
		RootCAs:    pool,
		ServerName: "trellis",
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
		CACert:     flagConfig.CACert,
		Cert:       flagConfig.Cert,
		Key:        flagConfig.Key,
		Output:     flagConfig.Output,
	}

	// 1. Run file — lowest priority; present only while a local node is running.
	if lc, err := localconfig.Read(localconfig.DefaultPath); err == nil {
		merged.ServerAddr = lc.ServerAddr
		merged.ClusterToken = lc.ClusterToken
		merged.CACertPEM = lc.CACert
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", localconfig.DefaultPath, err)
	}

	// 2. User config file. Legacy flat values are defaults; a selected named
	// context overlays them before environment variables and explicit flags.
	cfgPath, err := userConfigPath()
	if err != nil {
		return err
	}
	file, err := readUserConfig(cfgPath)
	if err == nil {
		if file.ServerAddr != nil {
			merged.ServerAddr = *file.ServerAddr
		}
		if file.ClusterToken != nil {
			merged.ClusterToken = *file.ClusterToken
		}
		if file.Namespace != nil {
			merged.Namespace = *file.Namespace
		}
		if file.CACert != nil {
			merged.CACertPEM = *file.CACert
		}

		selected := file.CurrentContext
		if value, ok := os.LookupEnv("TRELLIS_CONTEXT"); ok {
			selected = value
		}
		flags := cmd.Root().PersistentFlags()
		if flags.Changed("context") {
			selected = flagConfig.Context
		}
		if selected != "" {
			ctx, ok := file.Contexts[selected]
			if !ok {
				return fmt.Errorf("context %q is not defined in %s", selected, cfgPath)
			}
			merged.Context = selected
			merged.ServerAddr = ctx.ServerAddr
			merged.ClusterToken = ctx.ClusterToken
			merged.Namespace = ctx.Namespace
			merged.CACert = ""
			merged.CACertPEM = ctx.CACert
			merged.Cert = ctx.Cert
			merged.Key = ctx.Key
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config file %s: %w", cfgPath, err)
	} else {
		// A requested context cannot be resolved without a config file.
		selected := ""
		if value, ok := os.LookupEnv("TRELLIS_CONTEXT"); ok {
			selected = value
		}
		if cmd.Root().PersistentFlags().Changed("context") {
			selected = flagConfig.Context
		}
		if selected != "" {
			return fmt.Errorf("context %q is not defined: %s does not exist", selected, cfgPath)
		}
	}

	// 3. Environment variables.
	if value, ok := os.LookupEnv("TRELLIS_ADDR"); ok {
		merged.ServerAddr = value
	}
	if value, ok := os.LookupEnv("TRELLIS_TOKEN"); ok {
		merged.ClusterToken = value
	}
	if value, ok := os.LookupEnv("TRELLIS_NAMESPACE"); ok {
		merged.Namespace = value
	}
	if value, ok := os.LookupEnv("TRELLIS_CA_CERT"); ok {
		merged.CACert = value
	}
	if value, ok := os.LookupEnv("TRELLIS_CERT"); ok {
		merged.Cert = value
	}
	if value, ok := os.LookupEnv("TRELLIS_KEY"); ok {
		merged.Key = value
	}

	// 4. Explicit flags — highest priority.
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
	if flags.Changed("ca-cert") {
		merged.CACert = flagConfig.CACert
	}
	if flags.Changed("cert") {
		merged.Cert = flagConfig.Cert
	}
	if flags.Changed("key") {
		merged.Key = flagConfig.Key
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
