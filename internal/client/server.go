package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type ServerClient struct {
	baseURL string
	client  *client
}

type NodeInfo struct {
	ID     uuid.UUID
	Host   string
	Port   int
	CPU    int
	Memory int64
	OS     string
	Arch   string
}

type Heartbeat struct {
	NodeID    uuid.UUID `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}

func NewServerClient(token string, addr string) *ServerClient {
	client := &client{
		token:  token,
		client: &http.Client{},
	}

	return &ServerClient{
		baseURL: addr,
		client:  client,
	}
}

func (s *ServerClient) GetClusterStatus(ctx context.Context, placeholder string) {

}

func (s *ServerClient) ListNodes(ctx context.Context) (*api.NodeListResponse, error) {
	var responseData api.NodeListResponse

	err := s.client.request(ctx, http.MethodGet, "/v1/nodes", nil, &responseData)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	return &responseData, nil
}

func (s *ServerClient) RegisterNode(ctx context.Context, nodeInfo *NodeInfo) (*api.NodeRegistrationResponse, error) {
	requestData := &api.NodeRegistrationRequest{
		ID:     nodeInfo.ID,
		Host:   nodeInfo.Host,
		Port:   nodeInfo.Port,
		CPU:    nodeInfo.CPU,
		Memory: nodeInfo.Memory,
		OS:     nodeInfo.OS,
		Arch:   nodeInfo.Arch,
	}
	var responseData api.NodeRegistrationResponse

	err := s.client.request(ctx, http.MethodPost, "/v1/nodes", requestData, &responseData)
	if err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}

	return &responseData, nil
}

func (s *ServerClient) GetJob(ctx context.Context, placeholder string) {

}

func (s *ServerClient) ListJobs(ctx context.Context, placeholder string) {

}

func (s *ServerClient) SubmitJob(ctx context.Context, spec *spec.JobSpec) error {
	requestData := &api.JobRegistrationRequest{
		Spec: *spec,
	}

	err := s.client.request(ctx, http.MethodPost, "/v1/nodes", requestData, nil)
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}

	return nil
}

func (s *ServerClient) DeleteJob(ctx context.Context, placeholder string) {

}

func (s *ServerClient) SendHeartbeat(ctx context.Context, id uuid.UUID, heartbeat *Heartbeat) error {
	requestData := &api.HeartbeatRequest{
		NodeID:    heartbeat.NodeID,
		Timestamp: heartbeat.Timestamp,
	}
	url := fmt.Sprintf("/v1/nodes/%s/heartbeat", id)

	err := s.client.request(ctx, http.MethodPost, url, requestData, nil)
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}

	return nil
}
