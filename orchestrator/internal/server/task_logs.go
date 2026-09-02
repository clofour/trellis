package server

import (
	"context"
	"fmt"
	"io"
)

// AllocationTaskLogsForNamespace opens one task's logs after namespace validation.
func (s *Server) AllocationTaskLogsForNamespace(ctx context.Context, namespace, id, task string, follow bool, tail int) (io.ReadCloser, error) {
	s.mu.RLock()
	var found *Allocation
	for _, allocation := range s.allocations {
		if allocation.ID == id && allocation.Namespace == namespace {
			found = allocation
			break
		}
	}
	if found == nil || found.Node == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("allocation not found")
	}
	address := fmt.Sprintf("%s:%d", found.Node.Host, found.Node.Port)
	s.mu.RUnlock()
	return s.client.TaskLogs(ctx, address, id, task, follow, tail)
}
