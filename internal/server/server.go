package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/state"
)

type Server struct {
	state *state.StateController

	cluster *models.Cluster
}

func NewServer(state *state.StateController) *Server {
	return &Server{
		state: state,
	}
}

func (s *Server) Init(ctx context.Context) (string, error) {
	cluster, err := s.state.GetCluster(ctx)
	if err != nil {
		return "", fmt.Errorf("get cluster: %w", err)
	}

	if cluster != nil {
		s.cluster = cluster
		return "", nil
	}

	b := make([]byte, 32)
	rand.Read(b)

	token := base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	cluster = &models.Cluster{
		Hash: hashHex,
	}

	err = s.state.PutCluster(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("init cluster: %w", err)
	}

	s.cluster = cluster

	return token, nil
}

func (s *Server) ValidateAPIToken(ctx context.Context, token string) bool {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	return s.cluster.Hash == hashHex
}
