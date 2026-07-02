package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/agent"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/runtime"

	"github.com/spf13/cobra"
)

const shutdownTime = 10 * time.Second

func main() {
	config := &models.Config{}

	root := &cobra.Command{
		Use:   "trellis-agent",
		Short: "Trellis agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(config)
		},
	}

	flags := root.Flags()
	flags.StringVar(&config.ListenAddr, "listen", ":9100", "HTTP API listen address")
	flags.StringVar(&config.DataRoot, "data-root", "/var/lib/trellis/data", "Directory for local state and volumes")
	flags.StringVar(&config.ContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "Containerd socket path")
	flags.StringVar(&config.ConsulAddr, "consul-addr", "127.0.0.1:8500", "Consul agent address")

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func run(config *models.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	crt, err := runtime.NewContainerdRuntime(config.ContainerdSock)
	if err != nil {
		return fmt.Errorf("init runtime %s: %w", config.ContainerdSock, err)
	}

	hm := health.NewHealthManager(crt, nil)

	vm := agent.NewVolumeManager()

	// Consul

	// server := &http.Server{
	// 	Addr: runConfig.ListenAddr,
	// 	Handler:
	// }

	agent := agent.NewAgent(crt, hm, vm)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTime)
	defer cancel()

	return nil
}
