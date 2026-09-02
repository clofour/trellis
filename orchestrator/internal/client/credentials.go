package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/clofour/trellis/internal/api"
)

// CreateCredential asks the bootstrap administrator API to mint a scoped credential.
func (s *ServerClient) CreateCredential(ctx context.Context, request *api.CredentialCreateRequest) (*api.CredentialCreateResponse, error) {
	var response api.CredentialCreateResponse
	if err := s.client.request(ctx, http.MethodPost, s.address()+"/v1/credentials", request, &response); err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}
	return &response, nil
}
