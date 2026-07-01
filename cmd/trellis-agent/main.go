package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/spf13/cobra"
)

type RunConfig struct {
	ListenAddr     string
	DataRoot       string
	ContainerdSock string
	ConsulAddr     string
}

func main() {
	runConfig := &RunConfig{}

	root := &cobra.Command{
		Use:   "trellis-agent",
		Short: "Trellis agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(runConfig)
		},
	}

	flags := root.Flags()
	flags.StringVar(&runConfig.ListenAddr, "listen", ":9100", "HTTP API listen address")
	flags.StringVar(&runConfig.DataRoot, "data-root", "/var/lib/trellis/data", "Directory for local state and volumes")
	flags.StringVar(&runConfig.ContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "Containerd socket path")
	flags.StringVar(&runConfig.ConsulAddr, "consul-addr", "127.0.0.1:8500", "Consul agent address")

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func run(runConfig *RunConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	crt, err := runtime.New(runConfig.ContainerdSock)
	if err != nil {
		return fmt.Errorf("init runtime %s: %w", runConfig.ContainerdSock, err)
	}

	// Consul

	// server := &http.Server{
	// 	Addr: runConfig.ListenAddr,
	// 	Handler:
	// }
}
