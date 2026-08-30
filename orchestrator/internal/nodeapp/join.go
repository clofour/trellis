package nodeapp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/clofour/trellis/internal/api"
)

func joinClusterTLS(ctx context.Context, log *slog.Logger, joinAddr, clusterToken, serverID, raftAddr string) (*api.RaftJoinResponse, error) {
	body, err := json.Marshal(api.RaftJoinRequest{ID: serverID, RaftAddress: raftAddr})
	if err != nil {
		return nil, err
	}
	base := normalizeJoinAddress(joinAddr)
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}}}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/raft/join", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+clusterToken)
		resp, requestErr := httpClient.Do(req)
		if requestErr == nil {
			respBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				requestErr = readErr
			} else if resp.StatusCode == http.StatusOK {
				var joinResp api.RaftJoinResponse
				if err := json.Unmarshal(respBody, &joinResp); err != nil {
					return nil, fmt.Errorf("decode join response: %w", err)
				}
				log.Info("received TLS materials from cluster")
				return &joinResp, nil
			} else {
				requestErr = fmt.Errorf("join returned status %d", resp.StatusCode)
			}
		}
		if attempt >= 30 {
			return nil, requestErr
		}
		log.Warn("join attempt failed, retrying", "error", requestErr, "attempt", attempt+1)
		if err := waitJoinRetry(ctx, attempt); err != nil {
			return nil, err
		}
	}
}

func joinClusterRaft(ctx context.Context, log *slog.Logger, joinAddr, clusterToken, serverID, raftAddr string, tlsConfig *tls.Config) error {
	body, err := json.Marshal(api.RaftJoinRequest{ID: serverID, RaftAddress: raftAddr})
	if err != nil {
		return err
	}
	base := normalizeJoinAddress(joinAddr)
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/raft/join", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+clusterToken)
		resp, requestErr := httpClient.Do(req)
		if requestErr == nil {
			_, readErr := io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if readErr != nil {
				requestErr = readErr
			} else if resp.StatusCode == http.StatusOK {
				log.Info("joined cluster successfully")
				return nil
			} else {
				requestErr = fmt.Errorf("join returned status %d", resp.StatusCode)
			}
		}
		if attempt >= 30 {
			return requestErr
		}
		log.Warn("join attempt failed, retrying", "error", requestErr, "attempt", attempt+1)
		if err := waitJoinRetry(ctx, attempt); err != nil {
			return err
		}
	}
}

func normalizeJoinAddress(address string) string {
	if strings.Contains(address, "://") {
		return address
	}
	return "https://" + address
}

func waitJoinRetry(ctx context.Context, attempt int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(min(attempt+1, 5)) * time.Second):
		return nil
	}
}
