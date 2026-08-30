package nodeapp

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Config struct {
	AgentListen, AgentAdvertise, ServerListen, ServerAdvertise string
	RaftListen, RaftAdvertise, Join                            string
	DataDir, Cluster, ClusterToken, ContainerdSock             string
	WireGuardPool, WireGuardEndpoint                           string
	WireGuardPort                                              int
	DNSListen                                                  string
	CACert, CAKey, Cert, Key                                   string
	Labels                                                     []string
}

func (c *Config) validate() error {
	if c.ClusterToken == "" {
		return fmt.Errorf("--cluster-token is required")
	}
	if c.WireGuardPort < 1 || c.WireGuardPort > 65535 {
		return fmt.Errorf("--wireguard-port must be between 1 and 65535")
	}
	return nil
}

func (c *Config) defaultAdvertiseAddresses(hostname string) {
	if c.AgentAdvertise == "" {
		c.AgentAdvertise = net.JoinHostPort(hostname, "8127")
	}
	if c.ServerAdvertise == "" {
		c.ServerAdvertise = net.JoinHostPort(hostname, "8128")
	}
	if c.RaftAdvertise == "" {
		c.RaftAdvertise = net.JoinHostPort(hostname, "8129")
	}
}

func splitAddress(address string) (string, int, error) {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func parseLabels(raw []string) (map[string]string, error) {
	labels := make(map[string]string, len(raw))
	for _, value := range raw {
		key, labelValue, ok := strings.Cut(value, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("label %q must be in key=value form", value)
		}
		labels[key] = labelValue
	}
	return labels, nil
}

func acquireNodeID(dataDir string) (uuid.UUID, error) {
	path := filepath.Join(dataDir, "node-id")
	raw, err := os.ReadFile(path)
	if err == nil {
		id, parseErr := uuid.Parse(strings.TrimSpace(string(raw)))
		if parseErr != nil {
			return uuid.Nil, fmt.Errorf("parse node ID: %w", parseErr)
		}
		return id, nil
	}
	if !os.IsNotExist(err) {
		return uuid.Nil, fmt.Errorf("read node ID: %w", err)
	}
	id := uuid.New()
	if err := os.WriteFile(path, []byte(id.String()), 0o600); err != nil {
		return uuid.Nil, fmt.Errorf("write node ID: %w", err)
	}
	return id, nil
}
