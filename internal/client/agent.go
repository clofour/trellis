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

func (s *AgentClient) RunAllocation(ctx context.Context, address string, allocation *NodeInfo) (*api.NodeRegistrationResponse, error) {
	requestData := &api.AllocationRequest{}
	var responseData api.NodeRegistrationResponse

	err := s.client.request(ctx, http.MethodPost, "/v1/nodes", requestData, &responseData)
	if err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}

	return &responseData, nil
}

func (s *AgentClient) StopAllocation(ctx context.Context, address string, allocId string) error {
	err := s.client.request(ctx, http.MethodPost, address+"/v1/allocations/"+allocId, nil, nil)
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}

	return nil
}
