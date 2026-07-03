package agent

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/runtime"
)

type PortManager struct {
	runtime runtime.ContainerRuntime

	claims map[int]*models.Port

	min    int
	max    int
	cursor int
}

func NewPortManager(runtime runtime.ContainerRuntime, min int, max int, cursor int) *PortManager {
	if min == 0 {
		min = 20000
	}
	if max == 0 {
		max = 40000
	}

	return &PortManager{
		runtime: runtime,

		claims: make(map[int]*models.Port),

		min:    min,
		max:    max,
		cursor: min,
	}
}

func (p *PortManager) Check(hostPort int) (bool, error) {
	_, ok := p.claims[hostPort]
	if ok {
		return true, nil
	}

	addr := fmt.Sprintf(":%d", hostPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno == syscall.EADDRINUSE {
			return true, nil
		}

		return true, err
	}
	listener.Close()

	return false, nil
}

func (p *PortManager) Claim(portSpec models.PortSpec) (*models.Port, error) {
	hostPort := portSpec.HostPort
	if hostPort == 0 {

		for {

			taken, err := p.Check(p.cursor)

			if err != nil {
				return nil, err
			}

			if taken {
				p.cursor++
				continue
			}

			hostPort = p.cursor
			break

		}

	} else {

		taken, err := p.Check(hostPort)
		if err != nil {
			return nil, err
		} else if taken {
			return nil, fmt.Errorf("port %d taken", hostPort)
		}

	}

	port := &models.Port{
		HostPort:      hostPort,
		ContainerPort: portSpec.ContainerPort,
	}

	p.claims[hostPort] = port

	return port, nil
}

func (p *PortManager) Release(port models.Port) error {
	hostPort := port.HostPort

	_, ok := p.claims[hostPort]
	if !ok {
		return fmt.Errorf("unclaimed port %d", hostPort)
	}

	delete(p.claims, hostPort)

	return nil
}

func Restore() error {
	// Not Implemented
	return nil
}
