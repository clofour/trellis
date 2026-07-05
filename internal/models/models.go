package models

type Port struct {
	HostPort      int
	ContainerPort int
}

type Mount struct {
	HostPath      string
	ContainerPath string
}
