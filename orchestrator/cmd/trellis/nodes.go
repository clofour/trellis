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
	cmd.AddCommand(NewNodesUndrainCmd())
	cmd.AddCommand(NewNodesRemoveCmd())
	cmd.AddCommand(NewNodesLeadershipTransferCmd())

	return cmd
}

func NewNodesLeadershipTransferCmd() *cobra.Command {
	return &cobra.Command{Use: "transfer-leadership", Args: cobra.NoArgs, Short: "Transfer Raft leadership to another voter", RunE: func(cmd *cobra.Command, _ []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		if err := client.NewServerClient(config.ClusterToken, config.ServerAddr, tlsCfg).TransferLeadership(cmd.Context()); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Raft leadership transfer started.")
		return err
	}}
}

func NewNodesUndrainCmd() *cobra.Command {
	return &cobra.Command{Use: "undrain ID", Args: cobra.ExactArgs(1), Short: "Allow scheduling on a drained node", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := uuid.Parse(args[0])
		if err != nil {
			return fmt.Errorf("invalid node ID: %w", err)
		}
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		if err := client.NewServerClient(config.ClusterToken, config.ServerAddr, tlsCfg).UndrainNode(cmd.Context(), id); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Node un-drained.")
		return err
	}}
}

func NewNodesRemoveCmd() *cobra.Command {
	return &cobra.Command{Use: "remove ID", Args: cobra.ExactArgs(1), Short: "Permanently remove a node from the Raft cluster", RunE: func(cmd *cobra.Command, args []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		if err := client.NewServerClient(config.ClusterToken, config.ServerAddr, tlsCfg).RemoveRaftMember(cmd.Context(), args[0]); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Node permanently removed.")
		return err
	}}
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
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Node drain started.")
		return err
	}}
}

func NewNodesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List nodes in a cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "No nodes")
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)

			if _, err := fmt.Fprintln(w, "ID\tAddress\tStatus\tVersion\tCPU (m)\tMemory (bytes)\tHeartbeat"); err != nil {
				return err
			}

			for _, node := range *nodes {
				addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
				heartbeat := node.LastHeartbeat.Format(time.RFC3339)
				version := node.Version
				if version == "" {
					version = "unknown"
				}

				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n", node.ID, addr, node.Status, version, node.CPU, node.Memory, heartbeat); err != nil {
					return err
				}
			}

			return w.Flush()
		},
	}
}
