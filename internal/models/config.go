package models

type ServerConfig struct {
	ListenAddr string
	DataDir    string
}

type AgentConfig struct {
	ListenAddr     string
	DataDir        string
	ServerAddr     string
	ClusterToken   string
	ContainerdSock string
	ConsulAddr     string
}

type CLIConfig struct {
	ServerAddr   string
	ClusterToken string
}
