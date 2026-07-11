package client

import (
	"context"
	"net/http"
)

type ServerClient struct {
	token  string
	client *http.Client
}

func NewServerClient(token string) *ServerClient {
	return &ServerClient{
		token:  token,
		client: &http.Client{},
	}
}

func (s *ServerClient) GetClusterStatus(ctx context.Context, placeholder string) {

}

func (s *ServerClient) ListNodes(ctx context.Context, placeholder string) {

}

func (s *ServerClient) RegisterNode(ctx context.Context, placeholder string) {

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
