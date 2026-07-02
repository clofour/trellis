package models

type Config struct {
	ListenAddr     string
	DataRoot       string
	ContainerdSock string
	ConsulAddr     string
}

type Port struct {
	HostPort      int
	ContainerPort int
}

type Mount struct {
	HostPath      string
	ContainerPath string
}
