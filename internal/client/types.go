package client

import "github.com/google/uuid"

type NodeRegistrationRequest struct {
	ID     uuid.UUID
	Host   string
	Port   int
	CPU    int
	Memory int64
	OS     string
	Arch   string
}

type NodeRegistrationResponse struct {
	ID uuid.UUID
}
