package client

// ClusterToken returns the bearer token configured for agent requests.
// The server uses the same cluster-administrator credential for workloads that
// explicitly request cluster-scoped API access.
func (s *AgentClient) ClusterToken() string {
	if s == nil || s.client == nil {
		return ""
	}
	return s.client.token
}
