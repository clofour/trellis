package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
)

func isHTTPStatus(err error, status int) bool {
	var httpErr *client.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status
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
				return fmt.Errorf("timed out waiting for job %s to become healthy; run 'trellisctl jobs diagnose %s'", name, name)
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

type jobLogStream struct {
	allocation api.AllocationResponse
	task       string
}

func runJobLogs(ctx context.Context, w io.Writer, serverClient *client.ServerClient, target, allocationRef, group, task string, follow bool, tail int) error {
	streams, err := resolveLogStreams(ctx, serverClient, target, allocationRef, group, task)
	if err != nil {
		return err
	}
	if follow && len(streams) != 1 {
		return fmt.Errorf("--follow requires exactly one task stream; narrow with --allocation, --group, or --task (matches: %s)", logStreamRefs(streams))
	}

	for i, stream := range streams {
		if len(streams) > 1 {
			if _, err := fmt.Fprintf(w, "==> %s %s/%s <==\n", shortID(stream.allocation.ID), stream.allocation.Group, displayTask(stream.task)); err != nil {
				return err
			}
		}
		logs, err := serverClient.AllocationTaskLogs(ctx, stream.allocation.ID, stream.task, follow, tail)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, logs)
		closeErr := logs.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(streams) > 1 && i < len(streams)-1 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveLogStreams(ctx context.Context, serverClient *client.ServerClient, target, allocationRef, group, task string) ([]jobLogStream, error) {
	status, err := serverClient.GetJob(ctx, target)
	if err != nil {
		return nil, err
	}
	matches := append([]api.AllocationResponse(nil), status.Allocations...)
	matches = filterAllocations(matches, group)
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

	groupTasks := make(map[string][]string)
	if status.Spec != nil {
		for _, candidateGroup := range status.Spec.TaskGroups {
			for _, candidateTask := range candidateGroup.Tasks {
				groupTasks[candidateGroup.Name] = append(groupTasks[candidateGroup.Name], candidateTask.Name)
			}
		}
	}

	streams := make([]jobLogStream, 0)
	for _, allocation := range matches {
		tasks := groupTasks[allocation.Group]
		if task != "" {
			if containsTask(tasks, task) {
				streams = append(streams, jobLogStream{allocation: allocation, task: task})
			}
			continue
		}
		if len(tasks) == 0 {
			streams = append(streams, jobLogStream{allocation: allocation})
			continue
		}
		for _, taskName := range tasks {
			streams = append(streams, jobLogStream{allocation: allocation, task: taskName})
		}
	}
	if len(streams) == 0 {
		if task != "" {
			return nil, fmt.Errorf("job %s has no task %q matching the requested allocation/group filters", target, task)
		}
		return nil, fmt.Errorf("job %s has no log streams matching the requested filters", target)
	}
	return streams, nil
}

func containsTask(tasks []string, task string) bool {
	for _, candidate := range tasks {
		if candidate == task {
			return true
		}
	}
	return false
}

func displayTask(task string) string {
	if task == "" {
		return "task"
	}
	return task
}

func logStreamRefs(streams []jobLogStream) string {
	refs := make([]string, 0, len(streams))
	for _, stream := range streams {
		refs = append(refs, fmt.Sprintf("%s/%s/%s", shortID(stream.allocation.ID), stream.allocation.Group, displayTask(stream.task)))
	}
	sort.Strings(refs)
	return strings.Join(refs, ", ")
}

func filterAllocations(allocations []api.AllocationResponse, group string) []api.AllocationResponse {
	result := make([]api.AllocationResponse, 0, len(allocations))
	for _, allocation := range allocations {
		if group != "" && allocation.Group != group {
			continue
		}
		result = append(result, allocation)
	}
	return result
}

func resolveAllocationPrefix(allocations []api.AllocationResponse, ref string) (api.AllocationResponse, error) {
	var matches []api.AllocationResponse
	for _, allocation := range allocations {
		if allocation.ID == ref {
			return allocation, nil
		}
		if strings.HasPrefix(allocation.ID, ref) {
			matches = append(matches, allocation)
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
	for _, allocation := range allocations {
		refs = append(refs, shortID(allocation.ID))
	}
	sort.Strings(refs)
	return strings.Join(refs, ", ")
}
