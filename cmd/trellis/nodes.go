package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/clofour/trellis/internal/client"
	"github.com/spf13/cobra"
)

func NewNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage nodes in a cluster",
	}

	cmd.AddCommand(NewNodesListCmd())

	return cmd
}

func NewNodesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List nodes in a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverClient := client.NewServerClient(config.ClusterToken, config.ServerAddr)

			nodes, err := serverClient.ListNodes(cmd.Context())
			if err != nil {
				return fmt.Errorf("list nodes: %w", err)
			}

			if len(*nodes) == 0 {
				fmt.Println("No nodes")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

			fmt.Fprintln(w, "ID\tAddress\tStatus\tHeartbeat")

			for _, node := range *nodes {
				addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
				heartbeat := node.LastHeartbeat.Format(time.RFC3339)

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", node.ID, addr, node.Status, heartbeat)
			}

			return w.Flush()
		},
	}
}
