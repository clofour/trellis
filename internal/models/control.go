package models

import (
	"time"

	"github.com/google/uuid"
)

type Cluster struct {
	Hash string
}

type NodeStatus string

const (
	StatusHealthy   NodeStatus = "healthy"
	StatusUnhealthy NodeStatus = "unhealthy"
	StatusDraining  NodeStatus = "draining"
)

type Node struct {
	ID            uuid.UUID
	Host          string
	Port          int
	Status        NodeStatus
	Resources     []string
	LastHeartbeat time.Time
}
