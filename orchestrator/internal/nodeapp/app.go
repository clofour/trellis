package nodeapp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/agent"
	"github.com/clofour/trellis/internal/client"
	trellisdns "github.com/clofour/trellis/internal/dns"
	"github.com/clofour/trellis/internal/election"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/network"
	containerruntime "github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/server"
	"github.com/clofour/trellis/internal/state"
	"github.com/clofour/trellis/internal/storage"
	"github.com/clofour/trellis/internal/tlsutil"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

func Run(parent context.Context, cfg *Config) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	if err := cfg.validate(); err != nil {
		return err
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
	cfg.defaultAdvertiseAddresses(hostname)
	agentHost, agentPort, err := splitAddress(cfg.AgentAdvertise)
	if err != nil {
		return fmt.Errorf("agent advertise address: %w", err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	local := storage.NewLocalStorage(cfg.DataDir)
	if err := local.Init(); err != nil {
		return fmt.Errorf("init local storage: %w", err)
	}

	materials, err := loadOrBootstrapTLS(ctx, log, cfg, local)
	if err != nil {
		return fmt.Errorf("TLS bootstrap: %w", err)
	}
	peerTLS, err := tlsutil.PeerTLSConfig(materials)
	if err != nil {
		return fmt.Errorf("peer TLS config: %w", err)
	}
	agentServerTLS, err := tlsutil.ServerTLSConfig(materials)
	if err != nil {
		return fmt.Errorf("agent server TLS config: %w", err)
	}
	leaderServerTLS, err := tlsutil.LeaderTLSConfig(materials)
	if err != nil {
		return fmt.Errorf("leader server TLS config: %w", err)
	}
	clientTLS, err := tlsutil.ClientTLSConfig(materials)
	if err != nil {
		return fmt.Errorf("client TLS config: %w", err)
	}

	raftStore, err := state.NewRaftStore(state.RaftConfig{
		DataDir:   cfg.DataDir,
		BindAddr:  cfg.RaftListen,
		Advertise: cfg.RaftAdvertise,
		ServerID:  cfg.ServerAdvertise,
		Bootstrap: cfg.Join == "",
		TLS:       peerTLS,
	})
	if err != nil {
		return fmt.Errorf("init raft store: %w", err)
	}
	defer raftStore.Close()

	if cfg.Join != "" && !raftStore.HadExistingState() {
		log.Info("joining cluster", "address", cfg.Join)
		if err := joinClusterRaft(ctx, log, cfg.Join, cfg.ClusterToken, cfg.ServerAdvertise, raftStore.LocalAddr(), clientTLS); err != nil {
			return fmt.Errorf("join cluster: %w", err)
		}
	}

	stateCtl := server.NewStateController(raftStore, cfg.Cluster)
	control := server.NewServer(log, local, stateCtl, raftStore, cfg.Cluster, cfg.ServerAdvertise)
	control.SetClusterJoiner(raftStore)
	control.SetClientTLS(clientTLS)
	if err := control.SetNetworkPool(cfg.WireGuardPool); err != nil {
		return err
	}
	server.RegisterMetrics(control, prometheus.DefaultRegisterer)

	for attempt := 0; ; attempt++ {
		if _, err := control.InitWithToken(ctx, cfg.ClusterToken); err == nil {
			break
		} else if attempt >= 30 {
			return fmt.Errorf("initialize control plane: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	runtimeClient, err := containerruntime.NewContainerdRuntime(cfg.ContainerdSock)
	if err != nil {
		return fmt.Errorf("init runtime: %w", err)
	}
	defer func() {
		if err := runtimeClient.Close(); err != nil {
			log.Error("close runtime", "error", err)
		}
	}()

	healthMgr := health.NewHealthManager(log, runtimeClient, nil)
	restartCtl := agent.NewRestartController(runtimeClient, nil)
	leaderClient := client.NewServerClient(cfg.ClusterToken, "", clientTLS)
	ag := agent.NewAgent(log, runtimeClient, healthMgr, restartCtl, agent.NewPortManager(runtimeClient, 0, 0, 0), agent.NewVolumeManager(cfg.DataDir), leaderClient, id)
	ag.ConfigureDurability(local, cfg.Cluster)

	networkManager, err := network.NewAutomatedWireGuardManager(filepath.Join(cfg.DataDir, "network"), cfg.WireGuardPort)
	if err != nil {
		return fmt.Errorf("initialize WireGuard identity: %w", err)
	}
	dnsHost, dnsPort, err := splitAddress(cfg.DNSListen)
	if err != nil {
		return fmt.Errorf("dns listen address: %w", err)
	}
	if dnsHost == "" || dnsHost == "0.0.0.0" {
		dnsHost = "127.0.0.1"
	}
	ag.SetDNSServers([]string{net.JoinHostPort(dnsHost, strconv.Itoa(dnsPort))})
	ag.SetNetworkManager(networkManager)

	endpoint := cfg.WireGuardEndpoint
	if endpoint == "" {
		endpoint = net.JoinHostPort(agentHost, strconv.Itoa(cfg.WireGuardPort))
	} else if _, _, err := net.SplitHostPort(endpoint); err != nil {
		endpoint = net.JoinHostPort(endpoint, strconv.Itoa(cfg.WireGuardPort))
	}
	publicKey, err := networkManager.Identity()
	if err != nil {
		return err
	}
	ag.SetWireGuardIdentity(publicKey, endpoint)
	ag.SetAdvertiseAddress(agentHost, agentPort)

	var sysinfo syscall.Sysinfo_t
	memory := int64(0)
	if syscall.Sysinfo(&sysinfo) == nil {
		memory = int64(sysinfo.Totalram) * int64(sysinfo.Unit)
	}
	ag.SetResources(goruntime.NumCPU()*1000, memory, goruntime.GOOS, goruntime.GOARCH)
	if len(cfg.Labels) > 0 {
		labels, err := parseLabels(cfg.Labels)
		if err != nil {
			return err
		}
		ag.SetLabels(labels)
	}
	ag.Init(ctx)

	dnsResolver := trellisdns.NewResolver(log, leaderClient, trellisdns.DefaultDomain)
	go func() {
		if err := dnsResolver.Run(ctx, cfg.DNSListen); err != nil && ctx.Err() == nil {
			log.Error("dns resolver stopped", "error", err)
		}
	}()

	agentHTTP := echo.New()
	agentHTTP.Use(middleware.Recover(), clusterAuthMiddleware(cfg.ClusterToken))
	agent.NewHandler(ag).Register(agentHTTP)
	go func() {
		if err := (echo.StartConfig{Address: cfg.AgentListen, TLSConfig: agentServerTLS, GracefulTimeout: shutdownTime}).Start(ctx, agentHTTP); err != nil && ctx.Err() == nil {
			log.Error("agent API stopped", "error", err)
			cancel()
		}
	}()

	elector := election.NewRaftElector(raftStore.Raft(), election.Leader{NodeID: id, Address: cfg.ServerAdvertise})
	events := make(chan election.Event)
	go func() {
		if err := elector.Run(ctx, events); err != nil && ctx.Err() == nil {
			log.Error("leader election stopped", "error", err)
			cancel()
		}
	}()
	go watchLeader(ctx, log, elector, leaderClient)

	serveLeader := func(termCtx context.Context) error {
		leaderHTTP := echo.New()
		leaderHTTP.Use(middleware.Recover(), leaderAuthMiddleware(cfg.ClusterToken, control.TokenManager()))
		server.NewHandler(control).Register(leaderHTTP)
		return (echo.StartConfig{Address: cfg.ServerListen, TLSConfig: leaderServerTLS, GracefulTimeout: shutdownTime}).Start(termCtx, leaderHTTP)
	}
	return runLeaderLoop(ctx, log, cfg, id, control, events, serveLeader)
}
