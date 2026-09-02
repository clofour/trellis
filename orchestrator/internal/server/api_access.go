package server

import (
	"context"
	"fmt"

	"github.com/clofour/trellis/internal/spec"
)

func (s *Server) apiAccessToken(ctx context.Context, mode spec.APIAccessMode, namespace string) (string, error) {
	switch mode {
	case spec.APIAccessNamespace:
		if s.tokenManager == nil {
			return "", fmt.Errorf("namespace API access is unavailable")
		}
		token, err := s.tokenManager.GetOrCreateNamespaceToken(ctx, namespace)
		if err != nil {
			return "", fmt.Errorf("create namespace token: %w", err)
		}
		return token, nil
	case spec.APIAccessCluster:
		token := s.client.ClusterToken()
		if token == "" {
			return "", fmt.Errorf("cluster API access is unavailable")
		}
		return token, nil
	case spec.APIAccessNone:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported API access mode %q", mode)
	}
}
