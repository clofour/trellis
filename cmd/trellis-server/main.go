package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/server"
	"github.com/clofour/trellis/internal/state"
	"github.com/clofour/trellis/internal/storage"
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

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	storage := storage.NewLocalStorage(config.DataDir)
	err := storage.Init()
	if err != nil {
		return fmt.Errorf("init local storage: %w", err)
	}

	stateStore, err := state.NewConsulStore()
	if err != nil {
		return fmt.Errorf("init state store: %w", err)
	}

	stateCtl := state.NewStateController(stateStore, "default")

	s := server.NewServer(log, storage, stateCtl)

	_, err = s.Init(ctx)
	if err != nil {
		return fmt.Errorf("initalize server: %w", err)
	}

	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: "header:Authorization:Bearer ",
		Validator: func(c *echo.Context, key string, source middleware.ExtractorSource) (bool, error) {
			return s.ValidateAPIToken(ctx, key), nil
		},
	}))
	handler := server.NewHandler(s)
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

	return nil
}
