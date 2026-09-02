package server

import (
	"context"
	"testing"

	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/spec"
)

func TestAPIAccessTokenCluster(t *testing.T) {
	s := &Server{client: client.NewAgentClient("cluster-token", nil)}
	token, err := s.apiAccessToken(context.Background(), spec.APIAccessCluster, "default")
	if err != nil {
		t.Fatalf("cluster api access: %v", err)
	}
	if token != "cluster-token" {
		t.Fatalf("cluster token = %q, want cluster-token", token)
	}
}

func TestAPIAccessTokenNone(t *testing.T) {
	s := &Server{}
	token, err := s.apiAccessToken(context.Background(), spec.APIAccessNone, "default")
	if err != nil {
		t.Fatalf("disabled api access: %v", err)
	}
	if token != "" {
		t.Fatalf("disabled api access returned token %q", token)
	}
}
