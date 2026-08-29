package main

import (
	"os"

	"github.com/spf13/cobra"
)

type CLIConfig struct {
	ServerAddr   string
	ClusterToken string
	Tenant       string
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
	persistentFlags.StringVar(&config.Tenant, "tenant", "", "Tenant scope for isolated objects")

	root.AddCommand(NewJobsCmd())
	root.AddCommand(NewNodesCmd())

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}
