package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/agent"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/discovery"
	"github.com/clofour/trellis/internal/election"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/network"
	containerruntime "github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/server"
	"github.com/clofour/trellis/internal/state"
	"github.com/clofour/trellis/internal/storage"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/cobra"
)

const shutdownTime = 10 * time.Second

type config struct {
	AgentListen, AgentAdvertise, ServerListen, ServerAdvertise string
	DataDir, Cluster, ClusterToken, ContainerdSock, ConsulAddr string
	ElectionTTL                                                time.Duration
	NetworkConfigDir                                           string
}

func main() {
	cfg := &config{}
	root := &cobra.Command{Use: "trellis-node", Short: "Trellis node", RunE: func(cmd *cobra.Command, _ []string) error { return run(cmd.Context(), cfg) }}
	f := root.Flags()
	f.StringVar(&cfg.AgentListen, "agent-listen", ":8127", "Agent API listen address")
	f.StringVar(&cfg.AgentAdvertise, "agent-advertise", "", "Agent address advertised to the cluster")
	f.StringVar(&cfg.ServerListen, "server-listen", ":8128", "Leader API listen address")
	f.StringVar(&cfg.ServerAdvertise, "server-advertise", "", "Leader API address advertised to the cluster")
	f.StringVar(&cfg.DataDir, "data-dir", "/var/lib/trellis/data", "Directory for local state and volumes")
	f.StringVar(&cfg.Cluster, "cluster", "default", "Cluster name")
	f.StringVar(&cfg.ClusterToken, "cluster-token", "", "Shared cluster token")
	f.StringVar(&cfg.ContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "Containerd socket path")
	f.StringVar(&cfg.ConsulAddr, "consul-addr", "127.0.0.1:8500", "Consul address")
	f.DurationVar(&cfg.ElectionTTL, "election-ttl", 15*time.Second, "Leader election session TTL")
	f.StringVar(&cfg.NetworkConfigDir, "network-config-dir", "/etc/trellis/networks", "Directory containing administrator-managed WireGuard network definitions")
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(parent context.Context, cfg *config) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if cfg.ClusterToken == "" {
		return fmt.Errorf("--cluster-token is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	id, err := acquireNodeID(cfg.DataDir)
	if err != nil {
		return err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("hostname: %w", err)
	}
	if cfg.AgentAdvertise == "" {
		cfg.AgentAdvertise = net.JoinHostPort(hostname, "8127")
	}
	if cfg.ServerAdvertise == "" {
		cfg.ServerAdvertise = net.JoinHostPort(hostname, "8128")
	}
	agentHost, agentPort, err := splitAddress(cfg.AgentAdvertise)
	if err != nil {
		return fmt.Errorf("agent advertise address: %w", err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store, err := state.NewConsulStoreWithAddress(cfg.ConsulAddr)
	if err != nil {
		return fmt.Errorf("init Consul: %w", err)
	}
	local := storage.NewLocalStorage(cfg.DataDir)
	if err := local.Init(); err != nil {
		return fmt.Errorf("init local storage: %w", err)
	}
	stateCtl := server.NewStateController(store, cfg.Cluster)
	control := server.NewServer(log, local, stateCtl)
	if _, err := control.InitWithToken(ctx, cfg.ClusterToken); err != nil {
		return fmt.Errorf("initialize control plane: %w", err)
	}

	runtimeClient, err := containerruntime.NewContainerdRuntime(cfg.ContainerdSock)
	if err != nil {
		return fmt.Errorf("init runtime: %w", err)
	}
	healthMgr := health.NewHealthManager(log, runtimeClient, nil)
	restartCtl := agent.NewRestartController(runtimeClient, nil)
	registry, err := discovery.NewConsulRegistry()
	if err != nil {
		return fmt.Errorf("init service registry: %w", err)
	}
	leaderClient := client.NewServerClient(cfg.ClusterToken, "")
	ag := agent.NewAgent(log, runtimeClient, healthMgr, restartCtl, agent.NewPortManager(runtimeClient, 0, 0, 0), agent.NewVolumeManager(cfg.DataDir), registry, leaderClient, id)
	ag.SetNetworkManager(network.NewWireGuardManager(cfg.NetworkConfigDir))
	ag.SetAdvertiseAddress(agentHost, agentPort)
	var sysinfo syscall.Sysinfo_t
	memory := int64(0)
	if syscall.Sysinfo(&sysinfo) == nil {
		memory = int64(sysinfo.Totalram) * int64(sysinfo.Unit)
	}
	ag.SetResources(goruntime.NumCPU()*1000, memory, goruntime.GOOS, goruntime.GOARCH)
	if err := ag.Init(ctx); err != nil {
		return fmt.Errorf("initialize agent: %w", err)
	}

	agentHTTP := echo.New()
	agentHTTP.Use(middleware.Recover(), authMiddleware(cfg.ClusterToken))
	agent.NewHandler(ag).Register(agentHTTP)
	go func() {
		if err := (echo.StartConfig{Address: cfg.AgentListen, GracefulTimeout: shutdownTime}).Start(ctx, agentHTTP); err != nil && ctx.Err() == nil {
			log.Error("agent API stopped", "error", err)
			stop()
		}
	}()

	elector := election.New(store.Client(), cfg.Cluster, election.Leader{NodeID: id, Address: cfg.ServerAdvertise}, cfg.ElectionTTL)
	events := make(chan election.Event)
	go func() {
		if err := elector.Run(ctx, events); err != nil {
			log.Error("leader election stopped", "error", err)
			stop()
		}
	}()
	go watchLeader(ctx, log, elector, leaderClient)

	var leaderCancel context.CancelFunc
	var leaderDone <-chan struct{}
	for {
		select {
		case <-ctx.Done():
			if leaderCancel != nil {
				leaderCancel()
			}
			return nil
		case event := <-events:
			if event.Elected {
				if err := control.Reload(ctx); err != nil {
					log.Error("load leader state failed", "error", err)
					stop()
					continue
				}
				termCtx, cancel := context.WithCancel(ctx)
				leaderCancel = cancel
				log.Info("leadership acquired", "node_id", id, "address", cfg.ServerAdvertise)
				control.Run(termCtx)
				leaderHTTP := echo.New()
				leaderHTTP.Use(middleware.Recover(), authMiddleware(cfg.ClusterToken))
				server.NewHandler(control).Register(leaderHTTP)
				done := make(chan struct{})
				leaderDone = done
				go func() {
					defer close(done)
					if err := (echo.StartConfig{Address: cfg.ServerListen, GracefulTimeout: shutdownTime}).Start(termCtx, leaderHTTP); err != nil && termCtx.Err() == nil {
						log.Error("leader API stopped", "error", err)
						cancel()
					}
				}()
			} else if leaderCancel != nil {
				log.Warn("leadership lost", "node_id", id)
				leaderCancel()
				if leaderDone != nil {
					select {
					case <-leaderDone:
					case <-time.After(shutdownTime):
						log.Error("leader API shutdown timed out")
					}
				}
				leaderCancel = nil
				leaderDone = nil
			}
		}
	}
}

func watchLeader(ctx context.Context, log *slog.Logger, elector *election.Elector, target *client.ServerClient) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		leader, err := elector.Current(ctx)
		if err != nil {
			log.Error("discover leader failed", "error", err)
		} else if leader != nil {
			target.SetAddress(leader.Address)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func authMiddleware(token string) echo.MiddlewareFunc {
	return middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{KeyLookup: "header:Authorization:Bearer ", Validator: func(_ *echo.Context, key string, _ middleware.ExtractorSource) (bool, error) {
		return subtle.ConstantTimeCompare([]byte(key), []byte(token)) == 1, nil
	}})
}

func splitAddress(address string) (string, int, error) {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func acquireNodeID(dataDir string) (uuid.UUID, error) {
	path := filepath.Join(dataDir, "node-id")
	raw, err := os.ReadFile(path)
	if err == nil {
		id, parseErr := uuid.Parse(strings.TrimSpace(string(raw)))
		if parseErr != nil {
			return uuid.Nil, fmt.Errorf("parse node ID: %w", parseErr)
		}
		return id, nil
	}
	if !os.IsNotExist(err) {
		return uuid.Nil, fmt.Errorf("read node ID: %w", err)
	}
	id := uuid.New()
	if err := os.WriteFile(path, []byte(id.String()), 0o600); err != nil {
		return uuid.Nil, fmt.Errorf("write node ID: %w", err)
	}
	return id, nil
}
