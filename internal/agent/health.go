package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/clofour/trellis/internal/runtime"
)

type HealthManager struct {
	name string
}

const checkTimeout = 10

func CheckHTTP(ctx context.Context, addr string, port int, path string) (bool, error) {
	client := &http.Client{}
	url := fmt.Sprintf("http://%s:%d%s", addr, port, path)

	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("constructing request %s: %w", url, err)
	}

	response, err := client.Do(request)
	if err != nil {
		return false, fmt.Errorf("executing request %s: %w", url, err)
	}
	defer response.Body.Close()

	return response.StatusCode >= 200 && response.StatusCode < 300, nil
}

func CheckTCP(ctx context.Context, addr string) (bool, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, fmt.Errorf("executing request %s: %w", addr, err)
	}
	defer conn.Close()

	return true, nil
}

func CheckScript(ctx context.Context, c runtime.ContainerRuntime, containerID string, command []string) (bool, error) {
	code, err := c.Exec(ctx, containerID, command)
	if err != nil {
		return false, fmt.Errorf("executing command %s: %w", command, err)
	}

	return code == 0, nil
}
