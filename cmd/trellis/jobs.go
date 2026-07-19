package main

import (
	"fmt"
	"os"

	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/spec"
	"github.com/spf13/cobra"
)

func NewJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage jobs in a cluster",
	}

	cmd.AddCommand(NewJobsApplyCmd())

	return cmd
}

func NewJobsApplyCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a job manifest to a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read file %s: %w", path, err)
			}

			job, err := spec.ParseYAML(content)
			if err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}

			err = spec.Validate(job)
			if err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			serverClient := client.NewServerClient(config.ClusterToken, config.ServerAddr)

			err = serverClient.SubmitJob(cmd.Context(), job)
			if err != nil {
				return fmt.Errorf("submit job: %w", err)
			}

			fmt.Println("Job submitted successfully.")

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&path, "file", "trellis.yaml", "Manifest path")

	return cmd
}
