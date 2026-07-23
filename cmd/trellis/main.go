package main

import (
	"os"

	"github.com/spf13/cobra"
)

type CLIConfig struct {
	ServerAddr   string
	ClusterToken string
}

var config *CLIConfig

func main() {
	root := &cobra.Command{
		Use:   "trellis",
		Short: "Trellis CLI",
	}

	persistentFlags := root.PersistentFlags()
	persistentFlags.StringVar(&config.ServerAddr, "server-addr", "localhost:8127", "Server HTTP API listen address")
	persistentFlags.StringVar(&config.ClusterToken, "cluster-token", "", "Cluster token")

	root.AddCommand()

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}
