package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/server"
	"github.com/clofour/trellis/internal/state"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/cobra"
)

const shutdownTime = 10 * time.Second

func main() {
	config := &models.ServerConfig{}

	root := &cobra.Command{
		Use:   "trellis-server",
		Short: "Trellis server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(config)
		},
	}

	flags := root.Flags()
	flags.StringVar(&config.ListenAddr, "listen", ":9100", "HTTP API listen address")
	flags.StringVar(&config.DataDir, "data-dir", "/var/lib/trellis/data", "Directory for local state and volumes")

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func run(config *models.ServerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	ss, err := state.NewConsulStore()
	if err != nil {
		return fmt.Errorf("init state store: %w", err)
	}

	sc := state.NewStateController(ss, "default")

	s := server.NewServer(sc)

	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: "header:Authorization:Bearer ",
		Validator: func(c *echo.Context, key string, source middleware.ExtractorSource) (bool, error) {
			return s.ValidateAPIToken(ctx, key), nil
		},
	}))
	handler := server.NewHandler()
	handler.Register(e)
	sConfig := echo.StartConfig{
		Address:         config.ListenAddr,
		GracefulTimeout: shutdownTime,
	}
	go func() {
		err := sConfig.Start(ctx, e)
		if err != nil {
			// error
		}
	}()

	return nil
}
