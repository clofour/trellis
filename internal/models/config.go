package models

type ControlConfig struct {
	ListenAddr string
	DataRoot   string
}

type AgentConfig struct {
	ListenAddr     string
	DataRoot       string
	ContainerdSock string
	ConsulAddr     string
}
