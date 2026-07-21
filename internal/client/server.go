package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type ServerClient struct {
	baseURL string
	token   string
	client  *http.Client
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
	return &ServerClient{
		baseURL: addr,
		token:   token,
		client:  &http.Client{},
	}
}

func (s *ServerClient) GetClusterStatus(ctx context.Context, placeholder string) {

}

func (s *ServerClient) ListNodes(ctx context.Context) (*api.NodeListResponse, error) {
	var responseData api.NodeListResponse

	err := s.request(ctx, http.MethodGet, "/v1/nodes", nil, &responseData)
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

	err := s.request(ctx, http.MethodPost, "/v1/nodes", requestData, &responseData)
	if err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}

	return &responseData, nil
}

func (s *ServerClient) GetJob(ctx context.Context, placeholder string) {

}

func (s *ServerClient) ListJobs(ctx context.Context, placeholder string) {

}

func (s *ServerClient) SubmitJob(ctx context.Context, requestData *spec.JobSpec) error {
	err := s.request(ctx, http.MethodPost, "/v1/nodes", convertJob(requestData), nil)
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

	err := s.request(ctx, http.MethodPost, url, requestData, nil)
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}

	return nil
}

func (s *ServerClient) request(ctx context.Context, method string, path string, requestData any, responseData any) error {
	var requestBody *bytes.Reader
	if requestData != nil {
		requestBodyBytes, err := json.Marshal(requestData)
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		requestBody = bytes.NewReader(requestBodyBytes)
	}

	url := s.baseURL + path
	request, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return fmt.Errorf("constructing request %s: %w", url, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+s.token)

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("executing request %s: %w", url, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if checkStatusCode(response.StatusCode) {
		return fmt.Errorf("status %d", response.StatusCode)
	}

	if responseData != nil {
		err = json.Unmarshal(responseBody, &responseData)
		if err != nil {
			return fmt.Errorf("unmarshal json: %w", err)
		}
	}

	return nil
}

func checkStatusCode(statusCode int) bool {
	return statusCode < 200 || statusCode >= 300
}

func convertJob(jobSpec *spec.JobSpec) *api.JobRegistrationRequest {

	taskGroups := make([]api.TaskGroupRegistrationRequest, 0, len(jobSpec.TaskGroups))
	for _, taskGroup := range jobSpec.TaskGroups {

		tasks := make([]api.TaskRegistrationRequest, 0, len(taskGroup.Tasks))
		for _, task := range taskGroup.Tasks {

			ports := make([]api.PortRequest, 0, len(task.Ports))
			for _, port := range task.Ports {
				ports = append(ports, api.PortRequest{
					HostPort:      port.HostPort,
					ContainerPort: port.ContainerPort,
				})
			}
			volumes := make([]api.VolumeRequest, 0, len(task.Volumes))
			for _, volume := range task.Volumes {
				volumes = append(volumes, api.VolumeRequest{
					Name: volume.Name,
					Path: volume.Path,
				})
			}
			resources := api.ResourcesRequest{
				CPU:    task.Resources.CPU,
				Memory: task.Resources.Memory,
			}
			healthCheck := api.HealthCheckRequest{
				Type:    task.HealthCheck.Type,
				Port:    task.HealthCheck.Port,
				Path:    task.HealthCheck.Path,
				Command: task.HealthCheck.Command,
			}

			tasks = append(tasks, api.TaskRegistrationRequest{
				Name:        task.Name,
				Image:       task.Image,
				Env:         task.Env,
				Ports:       ports,
				Volumes:     volumes,
				Resources:   &resources,
				HealthCheck: &healthCheck,
			})
		}

		taskGroups = append(taskGroups, api.TaskGroupRegistrationRequest{
			Name:  taskGroup.Name,
			Count: taskGroup.Count,
			Tasks: tasks,
		})
	}

	return &api.JobRegistrationRequest{
		Name:       jobSpec.Name,
		TaskGroups: taskGroups,
	}
}
