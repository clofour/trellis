package discovery

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/api"
)

type ConsulRegistry struct {
	client *api.Client
}

func NewConsulRegistry() (*ConsulRegistry, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	return &ConsulRegistry{
		client: client,
	}, nil
}

func (c *ConsulRegistry) Register(ctx context.Context, id string, name string, addr string, port int) error {
	agent := c.client.Agent()

	registration := &api.AgentServiceRegistration{
		ID:      id,
		Name:    name,
		Address: addr,
		Port:    port,
	}

	err := agent.ServiceRegister(registration)
	if err != nil {
		return fmt.Errorf("register %s: %w", id, err)
	}

	return nil
}

func (c *ConsulRegistry) Deregister(ctx context.Context, id string) error {
	agent := c.client.Agent()

	err := agent.ServiceDeregister(id)
	if err != nil {
		return fmt.Errorf("deregister %s: %w", id, err)
	}

	return nil
}
