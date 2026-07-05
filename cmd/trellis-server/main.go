package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/agent"
	"github.com/clofour/trellis/internal/models"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/cobra"
)

const shutdownTime = 10 * time.Second

func main() {
	config := &models.ServerConfig{}

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

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func run(config *models.ServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	e := echo.New()
	e.Use(middleware.Recover())
	handler := agent.NewHandler(ag)
	handler.Register(e)
	sc := echo.StartConfig{
		Address:         config.ListenAddr,
		GracefulTimeout: shutdownTime,
	}
	go func() {
		err := sc.Start(ctx, e)
		if err != nil {
			// error
		}
	}()
}
