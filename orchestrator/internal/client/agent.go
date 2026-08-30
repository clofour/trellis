package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/clofour/trellis/internal/api"
)

type AgentClient struct {
	client *client
}

type AgentOperationError struct {
	Response api.OperationResponse
}

func (e *AgentOperationError) Error() string {
	return fmt.Sprintf("agent operation %s: %s", e.Response.Code, e.Response.Message)
}

func decodeOperationError(err error) error {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}
	var response api.OperationResponse
	if json.Unmarshal(httpErr.Body, &response) == nil && response.Code != "" {
		return &AgentOperationError{Response: response}
	}
	var wrapped struct {
		Message json.RawMessage `json:"message"`
	}
	if json.Unmarshal(httpErr.Body, &wrapped) == nil && len(wrapped.Message) > 0 {
		var inner api.OperationResponse
		if json.Unmarshal(wrapped.Message, &inner) == nil && inner.Code != "" {
			return &AgentOperationError{Response: inner}
		}
		var str string
		if json.Unmarshal(wrapped.Message, &str) == nil {
			if json.Unmarshal([]byte(str), &inner) == nil && inner.Code != "" {
				return &AgentOperationError{Response: inner}
			}
		}
	}
	return err
}

func (s *AgentClient) Logs(ctx context.Context, address, allocID string, follow bool, tail int) (io.ReadCloser, error) {
	query := url.Values{"follow": {fmt.Sprint(follow)}, "tail": {fmt.Sprint(tail)}}
	return s.client.stream(ctx, normalizeBaseURL(address)+"/v1/allocations/"+url.PathEscape(allocID)+"/logs?"+query.Encode())
}

func NewAgentClient(token string) *AgentClient {
	client := &client{
		token:  token,
		client: newHTTPClient(),
	}

	return &AgentClient{
		client: client,
	}
}

func (s *AgentClient) RunAllocation(ctx context.Context, address string, allocation *api.AllocationRequest) error {
	var response api.OperationResponse
	err := s.client.request(ctx, http.MethodPost, normalizeBaseURL(address)+"/v1/allocations", allocation, &response)
	if err != nil {
		return fmt.Errorf("run allocation: %w", decodeOperationError(err))
	}
	return nil
}

func (s *AgentClient) StopAllocation(ctx context.Context, address string, request *api.StopAllocationRequest) error {
	var response api.OperationResponse
	err := s.client.request(ctx, http.MethodDelete, normalizeBaseURL(address)+"/v1/allocations/"+url.PathEscape(request.AllocationID), request, &response)
	if err != nil {
		return fmt.Errorf("stop allocation: %w", decodeOperationError(err))
	}

	return nil
}
