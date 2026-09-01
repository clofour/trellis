package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/spec"
	"github.com/spf13/cobra"
)

func NewJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage desired jobs",
		Long:  "Manage desired jobs in the selected namespace. Apply YAML job manifests, inspect desired and runtime state, read allocation logs, and delete jobs.",
	}

	cmd.AddCommand(NewJobsApplyCmd())
	cmd.AddCommand(NewJobsListCmd())
	cmd.AddCommand(NewJobsStatusCmd())
	cmd.AddCommand(NewJobsDeleteCmd())
	cmd.AddCommand(NewJobsLogsCmd())

	return cmd
}

func NewJobsListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List jobs in the selected namespace", RunE: func(cmd *cobra.Command, _ []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		jobs, err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).ListJobs(cmd.Context())
		if err != nil {
			return err
		}
		if config.Output == "json" {
			return writeJSON(cmd.OutOrStdout(), jobs)
		}
		if len(*jobs) == 0 {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "No jobs")
			return err
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "Name\tDesired\tRunning\tHealthy\tRevision"); err != nil {
			return err
		}
		for _, job := range *jobs {
			if _, err := fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n", job.Name, job.Desired, job.Running, job.Healthy, job.Revision); err != nil {
				return err
			}
		}
		return w.Flush()
	}}
}

func NewJobsLogsCmd() *cobra.Command {
	var follow bool
	var tail int
	cmd := &cobra.Command{Use: "logs ALLOCATION", Args: cobra.ExactArgs(1), Short: "Show logs for an allocation", RunE: func(cmd *cobra.Command, args []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		logs, err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).AllocationLogs(cmd.Context(), args[0], follow, tail)
		if err != nil {
			return err
		}
		defer func() { _ = logs.Close() }()
		_, err = io.Copy(cmd.OutOrStdout(), logs)
		return err
	}}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow new log output")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of trailing lines (0 means all)")
	return cmd
}

func NewJobsStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status NAME", Args: cobra.ExactArgs(1), Short: "Inspect a job and its allocations", RunE: func(cmd *cobra.Command, args []string) error {
		tlsCfg, err := buildCLITLSConfig()
		if err != nil {
			return err
		}
		status, err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).GetJob(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if config.Output == "json" {
			return writeJSON(cmd.OutOrStdout(), status)
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Job: %s\nRevision: %d\nDesired: %d\nRunning: %d\nHealthy: %d\n", status.Name, status.Revision, status.Desired, status.Running, status.Healthy); err != nil {
			return err
		}
		if len(status.Allocations) == 0 {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Allocations: none")
			return err
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Allocations:"); err != nil {
			return err
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "ID\tTask group/Task\tNode\tLifecycle\tHealth\tGeneration"); err != nil {
			return err
		}
		for _, a := range status.Allocations {
			if _, err := fmt.Fprintf(w, "%s\t%s/%s\t%s\t%s\t%s\t%d\n", a.ID, a.Group, a.Task, a.NodeID, a.Phase, a.Health, a.Generation); err != nil {
				return err
			}
		}
		return w.Flush()
	}}
}

func NewJobsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete NAME",
		Aliases: []string{"destroy"},
		Args:    cobra.ExactArgs(1),
		Short:   "Delete a job and stop its allocations",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			if err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg).DeleteJob(cmd.Context(), args[0]); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted job %s.\n", args[0])
			return err
		},
	}
}

func NewJobsApplyCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML job manifest",
		Long:  "Apply a YAML job manifest to create a job or advance its desired-state revision.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read file %s: %w", path, err)
			}

			job, err := spec.ParseYAML(content)
			if err != nil {
				return fmt.Errorf("parse job manifest: %w", err)
			}

			err = spec.Validate(job)
			if err != nil {
				return fmt.Errorf("validate job manifest: %w", err)
			}

			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return fmt.Errorf("build TLS config: %w", err)
			}

			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, job.Namespace, tlsCfg)

			err = serverClient.SubmitJob(cmd.Context(), job)
			if err != nil {
				return fmt.Errorf("apply job: %w", err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Applied job %s.\n", job.Name)
			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&path, "file", "trellis.yaml", "YAML job manifest path")

	return cmd
}
