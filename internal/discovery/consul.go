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
	return NewConsulRegistryWithAddress("")
}

func NewConsulRegistryWithAddress(address string) (*ConsulRegistry, error) {
	config := api.DefaultConfig()
	if address != "" {
		config.Address = address
	}
	client, err := api.NewClient(config)
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

	err := agent.ServiceRegisterOpts(registration, api.ServiceRegisterOpts{}.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("register %s: %w", id, err)
	}

	return nil
}

func (c *ConsulRegistry) Deregister(ctx context.Context, id string) error {
	agent := c.client.Agent()

	err := agent.ServiceDeregisterOpts(id, (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return fmt.Errorf("deregister %s: %w", id, err)
	}

	return nil
}
