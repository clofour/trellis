package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/clofour/trellis/internal/api"
)

type AgentClient struct {
	client *client
}

func (s *AgentClient) Logs(ctx context.Context, address, allocID string, follow bool, tail int) (io.ReadCloser, error) {
	query := url.Values{"follow": {fmt.Sprint(follow)}, "tail": {fmt.Sprint(tail)}}
	return s.client.stream(ctx, normalizeBaseURL(address)+"/v1/allocations/"+url.PathEscape(allocID)+"/logs?"+query.Encode())
}

func NewAgentClient(token string, tlsConfig *tls.Config) *AgentClient {
	c := &client{
		token:  token,
		client: newHTTPClient(tlsConfig),
	}

	return &AgentClient{
		client: c,
	}
}

func (s *AgentClient) RunAllocation(ctx context.Context, address string, allocation *api.AllocationRequest) error {
	err := s.client.request(ctx, http.MethodPost, normalizeBaseURL(address)+"/v1/allocations", allocation, nil)
	if err != nil {
		return fmt.Errorf("run allocation: %w", err)
	}
	return nil
}

func (s *AgentClient) StopAllocation(ctx context.Context, address string, allocID string) error {
	err := s.client.request(ctx, http.MethodDelete, normalizeBaseURL(address)+"/v1/allocations/"+url.PathEscape(allocID), nil, nil)
	if err != nil {
		return fmt.Errorf("stop allocation: %w", err)
	}

	return nil
}
