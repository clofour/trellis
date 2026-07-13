package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ServerClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewServerClient(token string, addr string, port string) *ServerClient {
	return &ServerClient{
		baseURL: fmt.Sprintf("http://%s:%d", addr, port),
		token:   token,
		client:  &http.Client{},
	}
}

func (s *ServerClient) GetClusterStatus(ctx context.Context, placeholder string) {

}

func (s *ServerClient) ListNodes(ctx context.Context, placeholder string) {

}

func (s *ServerClient) RegisterNode(ctx context.Context, requestData *NodeRegistrationRequest) (*NodeRegistrationResponse, error) {
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	url := s.baseURL + "/nodes"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("constructing request %s: %w", url, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+s.token)

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("executing request %s: %w", url, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var responseData NodeRegistrationResponse
	err = json.Unmarshal(responseBody, &responseData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
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

func (s *ServerClient) SendHeartbeat(ctx context.Context, placeholder string) {

}
