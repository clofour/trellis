package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type ServerClient struct {
	baseURL string
	client  *client
	mu      sync.RWMutex
}

func (s *ServerClient) AllocationLogs(ctx context.Context, id string, follow bool, tail int) (io.ReadCloser, error) {
	return s.client.stream(ctx, fmt.Sprintf("%s/v1/allocations/%s/logs?follow=%t&tail=%d", s.address(), url.PathEscape(id), follow, tail))
}

func (s *ServerClient) SetAddress(addr string) {
	s.mu.Lock()
	s.baseURL = normalizeBaseURL(addr)
	s.mu.Unlock()
}

func (s *ServerClient) address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseURL
}

func (s *ServerClient) Ready() bool {
	return s.address() != ""
}

type NodeInfo struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	CPU                int
	Memory             int64
	OS                 string
	Arch               string
	WireGuardPublicKey string
	WireGuardEndpoint  string
}

type Heartbeat struct {
	NodeID      uuid.UUID              `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Allocations []api.AllocationStatus `json:"allocations,omitempty"`
}

func NewServerClient(token string, addr string, tlsConfig *tls.Config) *ServerClient {
	return NewNamespaceServerClient(token, addr, "", tlsConfig)
}

func NewNamespaceServerClient(token string, addr string, namespace string, tlsConfig *tls.Config) *ServerClient {
	baseURL := normalizeBaseURL(addr)
	c := &client{
		token:     token,
		namespace: namespace,
		client:    newHTTPClient(tlsConfig),
	}

	return &ServerClient{
		baseURL: baseURL,
		client:  c,
	}
}

func (s *ServerClient) ListNodes(ctx context.Context) (*api.NodeListResponse, error) {
	var responseData api.NodeListResponse

	err := s.client.request(ctx, http.MethodGet, s.address()+"/v1/nodes", nil, &responseData)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	return &responseData, nil
}

func (s *ServerClient) DrainNode(ctx context.Context, id uuid.UUID) error {
	if err := s.client.request(ctx, http.MethodPost, fmt.Sprintf("%s/v1/nodes/%s/drain", s.address(), id), nil, nil); err != nil {
		return fmt.Errorf("drain node: %w", err)
	}
	return nil
}

func (s *ServerClient) RegisterNode(ctx context.Context, nodeInfo *NodeInfo) (*api.NodeRegistrationResponse, error) {
	requestData := &api.NodeRegistrationRequest{
		ID:                 nodeInfo.ID,
		Host:               nodeInfo.Host,
		Port:               nodeInfo.Port,
		CPU:                nodeInfo.CPU,
		Memory:             nodeInfo.Memory,
		OS:                 nodeInfo.OS,
		Arch:               nodeInfo.Arch,
		WireGuardPublicKey: nodeInfo.WireGuardPublicKey,
		WireGuardEndpoint:  nodeInfo.WireGuardEndpoint,
	}
	var responseData api.NodeRegistrationResponse

	err := s.client.request(ctx, http.MethodPost, s.address()+"/v1/nodes", requestData, &responseData)
	if err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}

	return &responseData, nil
}

func (s *ServerClient) GetJob(ctx context.Context, name string) (*api.JobStatusResponse, error) {
	var response api.JobStatusResponse
	if err := s.client.request(ctx, http.MethodGet, s.address()+"/v1/jobs/"+url.PathEscape(name), nil, &response); err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &response, nil
}

func (s *ServerClient) ListJobs(ctx context.Context) (*api.JobListResponse, error) {
	var response api.JobListResponse
	if err := s.client.request(ctx, http.MethodGet, s.address()+"/v1/jobs", nil, &response); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	return &response, nil
}

func (s *ServerClient) SubmitJob(ctx context.Context, spec *spec.JobSpec) error {
	requestData := &api.JobRegistrationRequest{
		Spec: *spec,
	}

	err := s.client.request(ctx, http.MethodPost, s.address()+"/v1/jobs", requestData, nil)
	if err != nil {
		return fmt.Errorf("submit job: %w", err)
	}

	return nil
}

func (s *ServerClient) DeleteJob(ctx context.Context, name string) error {
	if err := s.client.request(ctx, http.MethodDelete, s.address()+"/v1/jobs/"+url.PathEscape(name), nil, nil); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return nil
}

func (s *ServerClient) ListDiscovery(ctx context.Context) (*api.ServiceListResponse, error) {
	var responseData api.ServiceListResponse
	if err := s.client.request(ctx, http.MethodGet, s.address()+"/v1/internal/discovery", nil, &responseData); err != nil {
		return nil, fmt.Errorf("list discovery records: %w", err)
	}
	return &responseData, nil
}

func (s *ServerClient) SendHeartbeat(ctx context.Context, id uuid.UUID, heartbeat *Heartbeat) (*api.HeartbeatResponse, error) {
	requestData := &api.HeartbeatRequest{
		NodeID:      heartbeat.NodeID,
		Timestamp:   heartbeat.Timestamp,
		Allocations: heartbeat.Allocations,
	}
	url := fmt.Sprintf("%s/v1/nodes/%s/heartbeat", s.address(), id)

	var response api.HeartbeatResponse
	err := s.client.request(ctx, http.MethodPost, url, requestData, &response)
	if err != nil {
		return nil, fmt.Errorf("send heartbeat: %w", err)
	}

	return &response, nil
}

func normalizeBaseURL(addr string) string {
	addr = strings.TrimRight(strings.TrimSpace(addr), "/")
	if addr == "" {
		return ""
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "https://" + addr
	}
	return addr
}
