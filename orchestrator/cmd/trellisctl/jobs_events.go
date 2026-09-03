package main

import (
	"github.com/clofour/trellis/internal/client"
	"github.com/spf13/cobra"
)

func NewJobsEventsCmd() *cobra.Command {
	var allocation string
	cmd := &cobra.Command{
		Use:   "events JOB",
		Args:  cobra.ExactArgs(1),
		Short: "Show allocation lifecycle history for a job",
		Long:  "Show the recorded lifecycle transitions for all allocations of a job. Use --allocation with the short reference from jobs status to inspect one allocation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg)
			events, err := loadJobEvents(cmd.Context(), serverClient, args[0], allocation)
			if err != nil {
				return err
			}
			if config.Output == "json" {
				return writeJSON(cmd.OutOrStdout(), events)
			}
			return printJobEvents(cmd.OutOrStdout(), events)
		},
	}
	cmd.Flags().StringVar(&allocation, "allocation", "", "Allocation ID or unique ID prefix")
	return cmd
}
