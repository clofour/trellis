package main

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/clofour/trellis/internal/client"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func NewNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage nodes in a cluster",
	}

	cmd.AddCommand(NewNodesListCmd())
	cmd.AddCommand(NewNodesDrainCmd())

	return cmd
}

func NewNodesDrainCmd() *cobra.Command {
	return &cobra.Command{Use: "drain ID", Args: cobra.ExactArgs(1), Short: "Stop scheduling on a node and migrate its allocations", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := uuid.Parse(args[0])
		if err != nil {
			return fmt.Errorf("invalid node ID: %w", err)
		}
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		if err := client.NewServerClient(config.ClusterToken, config.ServerAddr, tlsCfg).DrainNode(cmd.Context(), id); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Node drain started.")
		return nil
	}}
}

func NewNodesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List nodes in a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			serverClient := client.NewServerClient(config.ClusterToken, config.ServerAddr, tlsCfg)

			nodes, err := serverClient.ListNodes(cmd.Context())
			if err != nil {
				return fmt.Errorf("list nodes: %w", err)
			}
			if config.Output == "json" {
				return writeJSON(cmd.OutOrStdout(), nodes)
			}

			if len(*nodes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No nodes")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)

			fmt.Fprintln(w, "ID\tAddress\tStatus\tCPU (m)\tMemory (bytes)\tHeartbeat")

			for _, node := range *nodes {
				addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
				heartbeat := node.LastHeartbeat.Format(time.RFC3339)

				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n", node.ID, addr, node.Status, node.CPU, node.Memory, heartbeat)
			}

			return w.Flush()
		},
	}
}
