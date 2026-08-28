package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/clofour/trellis/internal/api"
)

type AgentClient struct {
	client *client
}

func NewAgentClient(token string) *AgentClient {
	client := &client{
		token:  token,
		client: &http.Client{},
	}

	return &AgentClient{
		client: client,
	}
}

func (s *AgentClient) RunAllocation(ctx context.Context, address string, allocation *api.AllocationRequest) error {
	err := s.client.request(ctx, http.MethodPost, normalizeBaseURL(address)+"/v1/allocations", allocation, nil)
	if err != nil {
		return fmt.Errorf("run allocation: %w", err)
	}
	return nil
}

func (s *AgentClient) StopAllocation(ctx context.Context, address string, allocId string) error {
	err := s.client.request(ctx, http.MethodDelete, normalizeBaseURL(address)+"/v1/allocations/"+allocId, nil, nil)
	if err != nil {
		return fmt.Errorf("stop allocation: %w", err)
	}

	return nil
}
