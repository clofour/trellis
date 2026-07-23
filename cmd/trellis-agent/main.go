package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/agent"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/discovery"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/runtime"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/spf13/cobra"
)

const shutdownTime = 10 * time.Second

type AgentConfig struct {
	ListenAddr     string
	DataDir        string
	ServerAddr     string
	ClusterToken   string
	ContainerdSock string
	ConsulAddr     string
}

func main() {
	config := &AgentConfig{}

	root := &cobra.Command{
		Use:   "trellis-agent",
		Short: "Trellis agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(config)
		},
	}

	flags := root.Flags()
	flags.StringVar(&config.ListenAddr, "listen", ":9100", "Agent HTTP API listen address")
	flags.StringVar(&config.DataDir, "data-dir", "/var/lib/trellis/data", "Directory for local state and volumes")
	flags.StringVar(&config.ServerAddr, "server-addr", "localhost:8127", "Server HTTP API listen address")
	flags.StringVar(&config.ClusterToken, "cluster-token", "", "Cluster token")
	flags.StringVar(&config.ContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "Containerd socket path")
	flags.StringVar(&config.ConsulAddr, "consul-addr", "127.0.0.1:8500", "Consul agent address")

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func run(config *AgentConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	err := initializeDataDir(config.DataDir)
	if err != nil {
		return fmt.Errorf("init data dir %s: %w", config.DataDir, err)
	}
	id, err := acquireNodeID(config.DataDir)
	if err != nil {
		return fmt.Errorf("acquire node id: %w", err)
	}

	runtime, err := runtime.NewContainerdRuntime(config.ContainerdSock)
	if err != nil {
		return fmt.Errorf("init runtime %s: %w", config.ContainerdSock, err)
	}

	healthMgr := health.NewHealthManager(log, runtime, nil)

	restartCtl := agent.NewRestartController(runtime, nil)

	volumeMgr := agent.NewVolumeManager()

	portMgr := agent.NewPortManager(runtime, 0, 0, 0)

	registry, err := discovery.NewConsulRegistry()
	if err != nil {
		return fmt.Errorf("init service registry %s: %w", "TBA", err)
	}

	serverClient := client.NewServerClient(config.ClusterToken, config.ServerAddr)

	ag := agent.NewAgent(log, runtime, healthMgr, restartCtl, portMgr, volumeMgr, registry, serverClient, id)
	ag.Init(ctx)

	e := echo.New()
	e.Use(middleware.Recover())
	handler := agent.NewHandler(ag)
	handler.Register(e)
	startCfg := echo.StartConfig{
		Address:         config.ListenAddr,
		GracefulTimeout: shutdownTime,
	}
	go func() {
		err := startCfg.Start(ctx, e)
		if err != nil {
			// error
		}
	}()

	<-ctx.Done()

	// shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTime)
	// defer cancel()

	return nil
}

func initializeDataDir(path string) error {
	err := os.MkdirAll(path, 0o755)
	if err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	return nil
}

func acquireNodeID(dataPath string) (uuid.UUID, error) {
	path := filepath.Join(dataPath, "node-id")

	data, err := os.ReadFile(path)
	if err == nil {
		processed := strings.TrimSpace(string(data))
		id, err := uuid.Parse(processed)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid uuid %s", processed)
		}

		return id, nil
	}

	if !os.IsNotExist(err) {
		return uuid.Nil, fmt.Errorf("read node ID: %w", err)
	}

	id := uuid.New()
	err = os.WriteFile(path, []byte(id.String()), 0o644)
	if err != nil {
		return uuid.Nil, fmt.Errorf("write node ID %s: %w", id, err)
	}

	return id, nil
}
