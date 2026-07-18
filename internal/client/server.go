package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/clofour/trellis/internal/api"
	"github.com/google/uuid"
)

type ServerClient struct {
	baseURL string
	token   string
	client  *http.Client
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

func (s *ServerClient) RegisterNode(ctx context.Context, requestData *api.NodeRegistrationRequest) (*api.NodeRegistrationResponse, error) {
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

func (s *ServerClient) SubmitJob(ctx context.Context, placeholder string) {

}

func (s *ServerClient) DeleteJob(ctx context.Context, placeholder string) {

}

func (s *ServerClient) SendHeartbeat(ctx context.Context, id uuid.UUID, requestData *api.HeartbeatRequest) error {
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
