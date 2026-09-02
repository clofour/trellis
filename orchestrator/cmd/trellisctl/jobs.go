package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
	"github.com/spf13/cobra"
)

func NewJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage desired jobs",
		Long:  "Validate and plan YAML manifests, apply desired jobs, observe convergence, diagnose runtime problems, read logs, and delete jobs in the selected namespace.",
	}
	cmd.AddCommand(NewJobsValidateCmd())
	cmd.AddCommand(NewJobsDiffCmd())
	cmd.AddCommand(NewJobsApplyCmd())
	cmd.AddCommand(NewJobsListCmd())
	cmd.AddCommand(NewJobsStatusCmd())
	cmd.AddCommand(NewJobsWatchCmd())
	cmd.AddCommand(NewJobsDiagnoseCmd())
	cmd.AddCommand(NewJobsLogsCmd())
	cmd.AddCommand(NewJobsDeleteCmd())
	return cmd
}

func NewJobsValidateCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a YAML job manifest without contacting a cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			job, err := readJobManifest(path)
			if err != nil {
				return err
			}
			if config.Output == "json" {
				return writeJSON(cmd.OutOrStdout(), job)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Valid manifest: %s/%s (%d task groups, %d desired allocations)\n", job.Namespace, job.Name, len(job.TaskGroups), desiredAllocations(job))
			return err
		},
	}
	cmd.Flags().StringVar(&path, "file", "trellis.yaml", "YAML job manifest path")
	return cmd
}

func NewJobsDiffCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:     "diff",
		Aliases: []string{"plan"},
		Short:   "Show the changes a manifest would apply",
		RunE: func(cmd *cobra.Command, _ []string) error {
			job, err := readJobManifest(path)
			if err != nil {
				return err
			}
			if err := ensureActiveNamespace(job); err != nil {
				return err
			}
			serverClient, err := jobClient(job.Namespace)
			if err != nil {
				return err
			}
			current, exists, err := getExistingJob(cmd.Context(), serverClient, job.Name)
			if err != nil {
				return err
			}
			return printJobPlan(cmd.OutOrStdout(), current, exists, job)
		},
	}
	cmd.Flags().StringVar(&path, "file", "trellis.yaml", "YAML job manifest path")
	return cmd
}

func NewJobsApplyCmd() *cobra.Command {
	var path string
	var dryRun bool
	var wait bool
	var timeout time.Duration
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML job manifest",
		Long:  "Validate and apply a YAML job manifest. Use --dry-run to preview semantic changes or --wait to follow the resulting revision until desired capacity is healthy.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRun && wait {
				return fmt.Errorf("--dry-run and --wait cannot be used together")
			}
			job, err := readJobManifest(path)
			if err != nil {
				return err
			}
			if err := ensureActiveNamespace(job); err != nil {
				return err
			}
			serverClient, err := jobClient(job.Namespace)
			if err != nil {
				return err
			}
			current, exists, err := getExistingJob(cmd.Context(), serverClient, job.Name)
			if err != nil {
				return err
			}
			changes := []manifestChange(nil)
			if exists && current.Spec != nil {
				changes = diffJobSpecs(current.Spec, job)
			}
			if dryRun {
				return printJobPlan(cmd.OutOrStdout(), current, exists, job)
			}
			if exists && current.Spec != nil && len(changes) == 0 {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Job %s/%s already matches the manifest (revision %d).\n", job.Namespace, job.Name, current.Revision); err != nil {
					return err
				}
				if wait {
					return waitForJob(cmd.Context(), cmd.OutOrStdout(), serverClient, job.Name, interval, timeout)
				}
				return nil
			}
			if err := serverClient.SubmitJob(cmd.Context(), job); err != nil {
				return fmt.Errorf("apply job: %w", err)
			}
			after, getErr := serverClient.GetJob(cmd.Context(), job.Name)
			switch {
			case getErr != nil:
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Applied job %s/%s.\n", job.Namespace, job.Name); err != nil {
					return err
				}
			case exists:
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Applied job %s/%s: revision %d -> %d.\n", job.Namespace, job.Name, current.Revision, after.Revision); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Created job %s/%s at revision %d.\n", job.Namespace, job.Name, after.Revision); err != nil {
					return err
				}
			}
			if wait {
				return waitForJob(cmd.Context(), cmd.OutOrStdout(), serverClient, job.Name, interval, timeout)
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&path, "file", "trellis.yaml", "YAML job manifest path")
	flags.BoolVar(&dryRun, "dry-run", false, "Validate and show the plan without changing the cluster")
	flags.BoolVarP(&wait, "wait", "w", false, "Wait until desired job capacity is healthy")
	flags.DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait (0 means no timeout)")
	flags.DurationVar(&interval, "interval", 2*time.Second, "Polling interval while waiting")
	return cmd
}

func NewJobsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List jobs in the selected namespace",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			if _, err := fmt.Fprintln(w, "Name\tState\tDesired\tRunning\tHealthy\tRevision"); err != nil {
				return err
			}
			for _, job := range *jobs {
				if _, err := fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\n", job.Name, jobState(&job), job.Desired, job.Running, job.Healthy, job.Revision); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}
}

func NewJobsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status NAME",
		Args:  cobra.ExactArgs(1),
		Short: "Inspect a job and its allocations",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			return printJobStatus(cmd.OutOrStdout(), status)
		},
	}
}

func NewJobsWatchCmd() *cobra.Command {
	var timeout time.Duration
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "watch NAME",
		Args:  cobra.ExactArgs(1),
		Short: "Watch a job converge to healthy desired capacity",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg)
			return waitForJob(cmd.Context(), cmd.OutOrStdout(), serverClient, args[0], interval, timeout)
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait (0 means no timeout)")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	return cmd
}

func NewJobsDiagnoseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diagnose NAME",
		Args:  cobra.ExactArgs(1),
		Short: "Explain why a job is not healthy",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			return printJobDiagnosis(cmd.OutOrStdout(), status)
		},
	}
}

func NewJobsLogsCmd() *cobra.Command {
	var follow bool
	var tail int
	var allocation string
	var group string
	cmd := &cobra.Command{
		Use:   "logs JOB",
		Args:  cobra.ExactArgs(1),
		Short: "Show logs for a job without requiring full allocation IDs",
		Long:  "Show logs for a job. For a job with multiple allocations, non-following output includes every matching allocation. Use --allocation with the short prefix shown by 'jobs status' to select one allocation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg)
			return runJobLogs(cmd.Context(), cmd.OutOrStdout(), serverClient, args[0], allocation, group, follow, tail)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&follow, "follow", "f", false, "Follow new log output")
	flags.IntVar(&tail, "tail", 100, "Number of trailing lines (0 means all)")
	flags.StringVar(&allocation, "allocation", "", "Allocation ID or unique ID prefix")
	flags.StringVar(&group, "group", "", "Only allocations for this task group")
	return cmd
}

func NewJobsDeleteCmd() *cobra.Command {
	var wait bool
	var timeout time.Duration
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "delete NAME",
		Args:  cobra.ExactArgs(1),
		Short: "Delete a job and stop its allocations",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg)
			if err := serverClient.DeleteJob(cmd.Context(), args[0]); err != nil {
				return err
			}
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted job %s.\n", args[0]); err != nil {
				return err
			}
			if wait {
				return waitForJobDeletion(cmd.Context(), cmd.OutOrStdout(), serverClient, args[0], interval, timeout)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait until the job is no longer present")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Maximum time to wait (0 means no timeout)")
	cmd.Flags().DurationVar(&interval, "interval", time.Second, "Polling interval while waiting")
	return cmd
}

func readJobManifest(path string) (*spec.JobSpec, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	job, err := spec.ParseYAML(content)
	if err != nil {
		return nil, fmt.Errorf("parse job manifest: %w", err)
	}
	if err := spec.Validate(job); err != nil {
		return nil, fmt.Errorf("validate job manifest: %w", err)
	}
	return job, nil
}

func ensureActiveNamespace(job *spec.JobSpec) error {
	if config.Namespace == "" || config.Namespace == job.Namespace {
		return nil
	}
	source := "active namespace"
	if config.Context != "" {
		source = fmt.Sprintf("context %q", config.Context)
	}
	return fmt.Errorf("manifest namespace %q does not match %s namespace %q; change the manifest or select the intended namespace", job.Namespace, source, config.Namespace)
}

func jobClient(namespace string) (*client.ServerClient, error) {
	tlsCfg, err := buildCLITLSConfig()
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}
	return client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, namespace, tlsCfg), nil
}

func printJobStatus(w io.Writer, status *api.JobStatusResponse) error {
	if _, err := fmt.Fprintf(w, "Job: %s\nRevision: %d\nState: %s\nDesired: %d\nRunning: %d\nHealthy: %d\n", status.Name, status.Revision, jobState(status), status.Desired, status.Running, status.Healthy); err != nil {
		return err
	}
	if len(status.Allocations) == 0 {
		_, err := fmt.Fprintln(w, "Allocations: none")
		return err
	}
	if _, err := fmt.Fprintln(w, "Allocations:"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "Allocation\tTask group\tNode\tLifecycle\tHealth\tDiagnostic"); err != nil {
		return err
	}
	for _, a := range status.Allocations {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", shortID(a.ID), a.Group, allocationNode(a), a.Phase, a.Health, diagnosticSummary(a)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printJobDiagnosis(w io.Writer, status *api.JobStatusResponse) error {
	if _, err := fmt.Fprintf(w, "Job %s: %s (%d desired, %d running, %d healthy)\n", status.Name, jobState(status), status.Desired, status.Running, status.Healthy); err != nil {
		return err
	}
	if jobReady(status) {
		_, err := fmt.Fprintln(w, "No runtime problems detected: desired capacity is healthy.")
		return err
	}
	if len(status.Allocations) == 0 {
		_, err := fmt.Fprintln(w, "No allocations have been created yet. Check schedulable node capacity, placement constraints, required host volumes, and node health.")
		return err
	}
	problems := 0
	for _, a := range status.Allocations {
		if !allocationNeedsAttention(a, status.Revision) {
			continue
		}
		problems++
		if _, err := fmt.Fprintf(w, "\n- %s %s on %s: lifecycle=%s health=%s\n", shortID(a.ID), a.Group, allocationNode(a), a.Phase, a.Health); err != nil {
			return err
		}
		if a.Reason != "" {
			if _, err := fmt.Fprintf(w, "  reason: %s\n", a.Reason); err != nil {
				return err
			}
		}
		if a.Message != "" {
			if _, err := fmt.Fprintf(w, "  message: %s\n", a.Message); err != nil {
				return err
			}
		}
		if a.NextRetryAt != nil {
			if _, err := fmt.Fprintf(w, "  next retry: %s\n", a.NextRetryAt.Format(time.RFC3339)); err != nil {
				return err
			}
		}
		if a.Attempt > 0 {
			if _, err := fmt.Fprintf(w, "  attempt: %d\n", a.Attempt); err != nil {
				return err
			}
		}
	}
	if problems == 0 {
		if _, err := fmt.Fprintln(w, "The job is still converging; no allocation reports a specific failure yet."); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\nInspect logs with: trellis jobs logs %s\nFollow convergence with: trellis jobs watch %s\n", status.Name, status.Name)
	return err
}

func allocationNeedsAttention(a api.AllocationResponse, currentRevision int) bool {
	if a.Draining && a.JobRevision < currentRevision && a.Health != "unhealthy" && a.Phase != "failed" && a.Phase != "lost" {
		return false
	}
	return a.Phase != "running" || a.Health != "healthy" || a.Reason != "" || a.Message != "" || a.NextRetryAt != nil
}

func diagnosticSummary(a api.AllocationResponse) string {
	if a.Reason != "" {
		return a.Reason
	}
	if a.Message != "" {
		return a.Message
	}
	if a.NextRetryAt != nil {
		return "retry scheduled"
	}
	return "—"
}

func allocationNode(a api.AllocationResponse) string {
	if a.Address != "" {
		return a.Address
	}
	if a.NodeID.String() == "00000000-0000-0000-0000-000000000000" {
		return "—"
	}
	return shortID(a.NodeID.String())
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func jobReady(status *api.JobStatusResponse) bool {
	if status.Desired <= 0 {
		return false
	}
	if len(status.Allocations) == 0 {
		return status.Running >= status.Desired && status.Healthy >= status.Desired
	}
	currentRunning, currentHealthy := 0, 0
	for _, a := range status.Allocations {
		if a.JobRevision != status.Revision || a.Draining {
			continue
		}
		if a.Phase == lifecycle.PhaseRunning {
			currentRunning++
		}
		if a.Phase == lifecycle.PhaseRunning && a.Health == lifecycle.HealthHealthy {
			currentHealthy++
		}
	}
	return currentRunning >= status.Desired && currentHealthy >= status.Desired
}

func jobState(status *api.JobStatusResponse) string {
	if jobReady(status) {
		return "ready"
	}
	for _, a := range status.Allocations {
		if a.Draining && a.JobRevision < status.Revision {
			continue
		}
		if a.Health == "unhealthy" || a.Phase == "failed" || a.Phase == "lost" {
			return "degraded"
		}
	}
	return "converging"
}

func desiredAllocations(job *spec.JobSpec) int {
	total := 0
	for _, group := range job.TaskGroups {
		total += group.Count
	}
	return total
}
