package models

type ServerConfig struct {
	ListenAddr string
	DataRoot   string
}

type AgentConfig struct {
	ListenAddr     string
	DataRoot       string
	ServerAddr     string
	ClusterToken   string
	ContainerdSock string
	ConsulAddr     string
}
