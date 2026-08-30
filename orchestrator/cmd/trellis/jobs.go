package main

import (
	"fmt"
	"io"
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
	cmd.AddCommand(NewJobsStatusCmd())
	cmd.AddCommand(NewJobsDestroyCmd())
	cmd.AddCommand(NewJobsLogsCmd())

	return cmd
}

func NewJobsLogsCmd() *cobra.Command {
	var follow bool
	var tail int
	cmd := &cobra.Command{Use: "logs ALLOCATION", Args: cobra.ExactArgs(1), Short: "Stream allocation logs", RunE: func(cmd *cobra.Command, args []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		logs, err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).AllocationLogs(cmd.Context(), args[0], follow, tail)
		if err != nil {
			return err
		}
		defer logs.Close()
		_, err = io.Copy(cmd.OutOrStdout(), logs)
		return err
	}}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow new log output")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of trailing lines (0 means all)")
	return cmd
}

func NewJobsStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status NAME", Args: cobra.ExactArgs(1), Short: "Show desired and actual job state", RunE: func(cmd *cobra.Command, args []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		status, err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).GetJob(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Job: %s\nRevision: %d\nDesired: %d\nRunning: %d\nHealthy: %d\n", status.Name, status.Revision, status.Desired, status.Running, status.Healthy)
		for _, a := range status.Allocations {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s/%s\t%s\t%s\n", a.ID, a.Group, a.Task, a.NodeID, a.Status)
		}
		return nil
	}}
}

func NewJobsDestroyCmd() *cobra.Command {
	return &cobra.Command{Use: "destroy NAME", Args: cobra.ExactArgs(1), Short: "Destroy a job and its allocations", RunE: func(cmd *cobra.Command, args []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		if err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).DeleteJob(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Job destroyed successfully.")
		return nil
	}}
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

			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return fmt.Errorf("build TLS config: %w", err)
			}

			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, job.Namespace, tlsCfg)

			err = serverClient.SubmitJob(cmd.Context(), job)
			if err != nil {
				return fmt.Errorf("submit job: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Job submitted successfully.")

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&path, "file", "trellis.yaml", "Manifest path")

	return cmd
}
