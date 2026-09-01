package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
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
			after, err := serverClient.GetJob(cmd.Context(), job.Name)
			if err != nil {
				if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Applied job %s/%s.\n", job.Namespace, job.Name); writeErr != nil {
					return writeErr
				}
			} else if exists {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Applied job %s/%s: revision %d -> %d.\n", job.Namespace, job.Name, current.Revision, after.Revision); err != nil {
					return err
				}
			} else {
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
	var task string

	cmd := &cobra.Command{
		Use:   "logs JOB_OR_ALLOCATION",
		Args:  cobra.ExactArgs(1),
		Short: "Show logs for a job without requiring full allocation IDs",
		Long:  "Show logs for a job or allocation. For a job with multiple allocations, non-following output includes every matching allocation. Use --allocation with the short prefix shown by 'jobs status' to select one allocation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsCfg, err := buildCLITLSConfig()
			if err != nil {
				return err
			}
			serverClient := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace, tlsCfg)
			selected, err := resolveLogAllocations(cmd.Context(), serverClient, args[0], allocation, group, task)
			if err != nil {
				return err
			}
			if follow && len(selected) != 1 {
				return fmt.Errorf("--follow requires exactly one allocation; select one with --allocation PREFIX, --group, or --task (matches: %s)", allocationRefs(selected))
			}
			for i, alloc := range selected {
				if len(selected) > 1 {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "==> %s %s/%s <==\n", shortID(alloc.ID), alloc.Group, displayTask(alloc.Task)); err != nil {
						return err
					}
				}
				logs, err := serverClient.AllocationLogs(cmd.Context(), alloc.ID, follow, tail)
				if err != nil {
					return err
				}
				_, copyErr := io.Copy(cmd.OutOrStdout(), logs)
				closeErr := logs.Close()
				if copyErr != nil {
					return copyErr
				}
				if closeErr != nil {
					return closeErr
				}
				if len(selected) > 1 && i < len(selected)-1 {
					if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&follow, "follow", "f", false, "Follow new log output")
	flags.IntVar(&tail, "tail", 100, "Number of trailing lines (0 means all)")
	flags.StringVar(&allocation, "allocation", "", "Allocation ID or unique ID prefix")
	flags.StringVar(&group, "group", "", "Only allocations for this task group")
	flags.StringVar(&task, "task", "", "Only allocations for this task")
	return cmd
}

func NewJobsDeleteCmd() *cobra.Command {
	var wait bool
	var timeout time.Duration
	var interval time.Duration
	cmd := &cobra.Command{
		Use:     "delete NAME",
		Aliases: []string{"destroy"},
		Args:    cobra.ExactArgs(1),
		Short:   "Delete a job and stop its allocations",
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

func getExistingJob(ctx context.Context, serverClient *client.ServerClient, name string) (*api.JobStatusResponse, bool, error) {
	status, err := serverClient.GetJob(ctx, name)
	if err == nil {
		return status, true, nil
	}
	if isHTTPStatus(err, http.StatusNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

func isHTTPStatus(err error, status int) bool {
	var httpErr *client.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status
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
	if _, err := fmt.Fprintln(tw, "Allocation\tTask group/Task\tNode\tLifecycle\tHealth\tDiagnostic"); err != nil {
		return err
	}
	for _, a := range status.Allocations {
		if _, err := fmt.Fprintf(tw, "%s\t%s/%s\t%s\t%s\t%s\t%s\n", shortID(a.ID), a.Group, displayTask(a.Task), allocationNode(a), a.Phase, a.Health, diagnosticSummary(a)); err != nil {
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
		if _, err := fmt.Fprintf(w, "\n- %s %s/%s on %s: lifecycle=%s health=%s\n", shortID(a.ID), a.Group, displayTask(a.Task), allocationNode(a), a.Phase, a.Health); err != nil {
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

func displayTask(task string) string {
	if task == "" {
		return "*"
	}
	return task
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func jobReady(status *api.JobStatusResponse) bool {
	return status.Desired > 0 && status.Running >= status.Desired && status.Healthy >= status.Desired
}

func jobState(status *api.JobStatusResponse) string {
	if jobReady(status) {
		return "ready"
	}
	for _, a := range status.Allocations {
		if a.Health == "unhealthy" || a.Phase == "failed" || a.Phase == "lost" {
			return "degraded"
		}
	}
	return "converging"
}

func waitForJob(parent context.Context, w io.Writer, serverClient *client.ServerClient, name string, interval, timeout time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be greater than zero")
	}
	ctx := parent
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()

	last := ""
	for {
		status, err := serverClient.GetJob(ctx, name)
		if err != nil {
			return err
		}
		line := fmt.Sprintf("revision %d: %d/%d running, %d/%d healthy (%s)", status.Revision, status.Running, status.Desired, status.Healthy, status.Desired, jobState(status))
		if line != last {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
			last = line
		}
		if jobReady(status) {
			_, err := fmt.Fprintf(w, "Ready: %d/%d allocations healthy.\n", status.Healthy, status.Desired)
			return err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timed out waiting for job %s to become healthy; run 'trellis jobs diagnose %s'", name, name)
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func waitForJobDeletion(parent context.Context, w io.Writer, serverClient *client.ServerClient, name string, interval, timeout time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be greater than zero")
	}
	ctx := parent
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()
	for {
		_, err := serverClient.GetJob(ctx, name)
		if isHTTPStatus(err, http.StatusNotFound) {
			_, err = fmt.Fprintln(w, "Job is no longer present.")
			return err
		}
		if err != nil {
			return err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timed out waiting for job %s to disappear", name)
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func resolveLogAllocations(ctx context.Context, serverClient *client.ServerClient, target, allocationRef, group, task string) ([]api.AllocationResponse, error) {
	status, err := serverClient.GetJob(ctx, target)
	if err == nil {
		matches := append([]api.AllocationResponse(nil), status.Allocations...)
		matches = filterAllocations(matches, group, task)
		if allocationRef != "" {
			resolved, err := resolveAllocationPrefix(matches, allocationRef)
			if err != nil {
				return nil, err
			}
			matches = []api.AllocationResponse{resolved}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("job %s has no allocations matching the requested filters", target)
		}
		return matches, nil
	}
	if !isHTTPStatus(err, http.StatusNotFound) {
		return nil, err
	}
	if allocationRef != "" || group != "" || task != "" {
		return nil, fmt.Errorf("%q is not a job; --allocation, --group, and --task require a job target", target)
	}
	allocations, err := serverClient.ListAllocations(ctx, "")
	if err != nil {
		return nil, err
	}
	resolved, err := resolveAllocationPrefix(*allocations, target)
	if err != nil {
		return nil, fmt.Errorf("%q is neither a visible job nor a unique allocation ID/prefix: %w", target, err)
	}
	return []api.AllocationResponse{resolved}, nil
}

func filterAllocations(allocations []api.AllocationResponse, group, task string) []api.AllocationResponse {
	result := make([]api.AllocationResponse, 0, len(allocations))
	for _, a := range allocations {
		if group != "" && a.Group != group {
			continue
		}
		if task != "" && a.Task != task {
			continue
		}
		result = append(result, a)
	}
	return result
}

func resolveAllocationPrefix(allocations []api.AllocationResponse, ref string) (api.AllocationResponse, error) {
	var matches []api.AllocationResponse
	for _, a := range allocations {
		if a.ID == ref {
			return a, nil
		}
		if strings.HasPrefix(a.ID, ref) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return api.AllocationResponse{}, fmt.Errorf("no allocation matches %q", ref)
	}
	return api.AllocationResponse{}, fmt.Errorf("allocation prefix %q is ambiguous (%s)", ref, allocationRefs(matches))
}

func allocationRefs(allocations []api.AllocationResponse) string {
	refs := make([]string, 0, len(allocations))
	for _, a := range allocations {
		refs = append(refs, shortID(a.ID))
	}
	sort.Strings(refs)
	return strings.Join(refs, ", ")
}

func desiredAllocations(job *spec.JobSpec) int {
	total := 0
	for _, group := range job.TaskGroups {
		total += group.Count
	}
	return total
}

type manifestChange struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

func printJobPlan(w io.Writer, current *api.JobStatusResponse, exists bool, desired *spec.JobSpec) error {
	if !exists || current == nil || current.Spec == nil {
		plan := struct {
			Action             string `json:"action"`
			Namespace          string `json:"namespace"`
			Job                string `json:"job"`
			DesiredAllocations int    `json:"desired_allocations"`
		}{"create", desired.Namespace, desired.Name, desiredAllocations(desired)}
		if config.Output == "json" {
			return writeJSON(w, plan)
		}
		_, err := fmt.Fprintf(w, "Plan: create %s/%s (%d desired allocations)\n", desired.Namespace, desired.Name, plan.DesiredAllocations)
		return err
	}
	changes := diffJobSpecs(current.Spec, desired)
	if config.Output == "json" {
		action := "update"
		if len(changes) == 0 {
			action = "none"
		}
		return writeJSON(w, struct {
			Action          string           `json:"action"`
			Namespace       string           `json:"namespace"`
			Job             string           `json:"job"`
			CurrentRevision int              `json:"current_revision"`
			Changes         []manifestChange `json:"changes"`
		}{action, desired.Namespace, desired.Name, current.Revision, changes})
	}
	if len(changes) == 0 {
		_, err := fmt.Fprintf(w, "Plan: no changes to %s/%s (revision %d)\n", desired.Namespace, desired.Name, current.Revision)
		return err
	}
	if _, err := fmt.Fprintf(w, "Plan: update %s/%s from revision %d\n", desired.Namespace, desired.Name, current.Revision); err != nil {
		return err
	}
	for _, change := range changes {
		switch change.Operation {
		case "add":
			if _, err := fmt.Fprintf(w, "  + %s: %s\n", change.Path, formatChangeValue(change.Path, change.After)); err != nil {
				return err
			}
		case "remove":
			if _, err := fmt.Fprintf(w, "  - %s: %s\n", change.Path, formatChangeValue(change.Path, change.Before)); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(w, "  ~ %s: %s -> %s\n", change.Path, formatChangeValue(change.Path, change.Before), formatChangeValue(change.Path, change.After)); err != nil {
				return err
			}
		}
	}
	return nil
}

func diffJobSpecs(before, after *spec.JobSpec) []manifestChange {
	var left any
	var right any
	leftRaw, _ := json.Marshal(before)
	rightRaw, _ := json.Marshal(after)
	_ = json.Unmarshal(leftRaw, &left)
	_ = json.Unmarshal(rightRaw, &right)
	changes := make([]manifestChange, 0)
	walkManifestDiff("", left, right, &changes)
	return changes
}

func walkManifestDiff(path string, before, after any, changes *[]manifestChange) {
	if reflect.DeepEqual(before, after) {
		return
	}
	leftMap, leftOK := before.(map[string]any)
	rightMap, rightOK := after.(map[string]any)
	if leftOK && rightOK {
		keys := make(map[string]struct{}, len(leftMap)+len(rightMap))
		for key := range leftMap {
			keys[key] = struct{}{}
		}
		for key := range rightMap {
			keys[key] = struct{}{}
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			left, leftExists := leftMap[key]
			right, rightExists := rightMap[key]
			child := joinManifestPath(path, key)
			switch {
			case !leftExists:
				*changes = append(*changes, manifestChange{Operation: "add", Path: child, After: right})
			case !rightExists:
				*changes = append(*changes, manifestChange{Operation: "remove", Path: child, Before: left})
			default:
				walkManifestDiff(child, left, right, changes)
			}
		}
		return
	}
	leftSlice, leftOK := before.([]any)
	rightSlice, rightOK := after.([]any)
	if leftOK && rightOK {
		if leftNamed, okLeft := namedManifestSlice(leftSlice); okLeft {
			if rightNamed, okRight := namedManifestSlice(rightSlice); okRight {
				names := make(map[string]struct{}, len(leftNamed)+len(rightNamed))
				for name := range leftNamed {
					names[name] = struct{}{}
				}
				for name := range rightNamed {
					names[name] = struct{}{}
				}
				ordered := make([]string, 0, len(names))
				for name := range names {
					ordered = append(ordered, name)
				}
				sort.Strings(ordered)
				for _, name := range ordered {
					left, leftExists := leftNamed[name]
					right, rightExists := rightNamed[name]
					child := fmt.Sprintf("%s[%s]", path, name)
					switch {
					case !leftExists:
						*changes = append(*changes, manifestChange{Operation: "add", Path: child, After: right})
					case !rightExists:
						*changes = append(*changes, manifestChange{Operation: "remove", Path: child, Before: left})
					default:
						walkManifestDiff(child, left, right, changes)
					}
				}
				return
			}
		}
		max := len(leftSlice)
		if len(rightSlice) > max {
			max = len(rightSlice)
		}
		for i := 0; i < max; i++ {
			child := fmt.Sprintf("%s[%d]", path, i)
			switch {
			case i >= len(leftSlice):
				*changes = append(*changes, manifestChange{Operation: "add", Path: child, After: rightSlice[i]})
			case i >= len(rightSlice):
				*changes = append(*changes, manifestChange{Operation: "remove", Path: child, Before: leftSlice[i]})
			default:
				walkManifestDiff(child, leftSlice[i], rightSlice[i], changes)
			}
		}
		return
	}
	*changes = append(*changes, manifestChange{Operation: "change", Path: path, Before: before, After: after})
}

func namedManifestSlice(values []any) (map[string]any, bool) {
	result := make(map[string]any, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		name, ok := item["name"].(string)
		if !ok || name == "" {
			return nil, false
		}
		if _, duplicate := result[name]; duplicate {
			return nil, false
		}
		result[name] = value
	}
	return result, true
}

func joinManifestPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func formatChangeValue(path string, value any) string {
	if value == nil {
		return "null"
	}
	if number, ok := value.(float64); ok && isDurationPath(path) {
		return time.Duration(int64(number)).String()
	}
	switch typed := value.(type) {
	case string:
		return fmt.Sprintf("%q", typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(raw)
	}
}

func isDurationPath(path string) bool {
	return strings.HasSuffix(path, ".window") || strings.HasSuffix(path, ".interval") || strings.HasSuffix(path, ".timeout")
}
