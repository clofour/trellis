package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/state"
	"github.com/clofour/trellis/internal/storage"
)

type Server struct {
	log     *slog.Logger
	storage *storage.LocalStorage
	state   *state.StateController

	cluster *models.Cluster
}

func NewServer(log *slog.Logger, storage *storage.LocalStorage, state *state.StateController) *Server {
	return &Server{
		log:     log.With("component", "server"),
		storage: storage,
		state:   state,
	}
}

func (s *Server) Init(ctx context.Context) (string, error) {
	cluster, err := s.state.GetCluster(ctx)
	if err != nil {
		return "", fmt.Errorf("get cluster: %w", err)
	}

	if cluster != nil {
		s.log.Info("cluster already initialized")

		s.cluster = cluster
		return "", nil
	}

	b := make([]byte, 32)
	rand.Read(b)

	token := base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	err = s.storage.Put("token", hashHex)
	if err != nil {
		return "", fmt.Errorf("save cluster locally: %w", err)
	}

	cluster = &models.Cluster{
		Hash: hashHex,
	}

	err = s.state.PutCluster(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("save cluster remotely: %w", err)
	}

	s.cluster = cluster

	return token, nil
}

func (s *Server) ValidateAPIToken(ctx context.Context, token string) bool {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	return subtle.ConstantTimeCompare([]byte(token), []byte(hashHex)) == 1
}
