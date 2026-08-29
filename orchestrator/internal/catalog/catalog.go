package catalog

import (
	"sync"

	"github.com/clofour/trellis/internal/api"
)

type ServiceInstance struct {
	ID      string
	Job     string
	Group   string
	Address string
	Ports   []api.PortMapping
	Labels  map[string]string
}

type ServiceCatalog struct {
	mu       sync.RWMutex
	services map[string][]ServiceInstance // keyed by namespace
}

func New() *ServiceCatalog {
	return &ServiceCatalog{
		services: make(map[string][]ServiceInstance),
	}
}

func (c *ServiceCatalog) Update(namespace string, instances []ServiceInstance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(instances) == 0 {
		delete(c.services, namespace)
	} else {
		c.services[namespace] = instances
	}
}

func (c *ServiceCatalog) Lookup(namespace, jobName string) []ServiceInstance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []ServiceInstance
	for _, inst := range c.services[namespace] {
		if inst.Job == jobName {
			result = append(result, inst)
		}
	}
	return result
}

type ListFilter struct {
	Job   string
	Label string // "key:value" format
}

// List returns service entries. If namespace is empty, entries from all
// namespaces are returned (requires cluster-level authorization).
func (c *ServiceCatalog) List(namespace string, filter *ListFilter) []api.ServiceEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []api.ServiceEntry
	for ns, instances := range c.services {
		if namespace != "" && ns != namespace {
			continue
		}
		for _, inst := range instances {
			if filter != nil && filter.Job != "" && inst.Job != filter.Job {
				continue
			}
			if filter != nil && filter.Label != "" && !matchLabel(inst.Labels, filter.Label) {
				continue
			}
			result = append(result, api.ServiceEntry{
				ID:        inst.ID,
				Job:       inst.Job,
				Group:     inst.Group,
				Namespace: ns,
				Labels:    inst.Labels,
				Address:   inst.Address,
				Ports:     inst.Ports,
				Status:    "healthy",
			})
		}
	}
	return result
}

func matchLabel(labels map[string]string, filter string) bool {
	for i := range filter {
		if filter[i] == ':' {
			key, value := filter[:i], filter[i+1:]
			return labels[key] == value
		}
	}
	_, ok := labels[filter]
	return ok
}
