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

func runJobLogs(ctx context.Context, w io.Writer, serverClient *client.ServerClient, target, allocationRef, group string, follow bool, tail int) error {
	selected, err := resolveLogAllocations(ctx, serverClient, target, allocationRef, group)
	if err != nil {
		return err
	}
	if follow && len(selected) != 1 {
		return fmt.Errorf("--follow requires exactly one allocation; select one with --allocation PREFIX or --group (matches: %s)", allocationRefs(selected))
	}

	for i, allocation := range selected {
		if len(selected) > 1 {
			if _, err := fmt.Fprintf(w, "==> %s %s <==\n", shortID(allocation.ID), allocation.Group); err != nil {
				return err
			}
		}
		logs, err := serverClient.AllocationLogs(ctx, allocation.ID, follow, tail)
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
		if len(selected) > 1 && i < len(selected)-1 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveLogAllocations(ctx context.Context, serverClient *client.ServerClient, target, allocationRef, group string) ([]api.AllocationResponse, error) {
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
	return matches, nil
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
