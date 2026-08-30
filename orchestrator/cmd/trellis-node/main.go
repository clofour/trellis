package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/clofour/trellis/internal/nodeapp"
	"github.com/spf13/cobra"
)

func main() {
	cfg := &nodeapp.Config{}
	root := &cobra.Command{Use: "trellis-node", Short: "Trellis node", RunE: func(cmd *cobra.Command, _ []string) error { return nodeapp.Run(cmd.Context(), cfg) }}
	f := root.Flags()
	f.StringVar(&cfg.AgentListen, "agent-listen", ":8127", "Agent API listen address")
	f.StringVar(&cfg.AgentAdvertise, "agent-advertise", "", "Agent address advertised to the cluster")
	f.StringVar(&cfg.ServerListen, "server-listen", ":8128", "Leader API listen address")
	f.StringVar(&cfg.ServerAdvertise, "server-advertise", "", "Leader API address advertised to the cluster")
	f.StringVar(&cfg.RaftListen, "raft-listen", ":8129", "Raft consensus transport listen address")
	f.StringVar(&cfg.RaftAdvertise, "raft-advertise", "", "Raft consensus transport advertised address")
	f.StringVar(&cfg.Join, "join", "", "Address of an existing cluster member to join (server API address)")
	f.StringVar(&cfg.DataDir, "data-dir", "/var/lib/trellis/data", "Directory for local state and volumes")
	f.StringVar(&cfg.Cluster, "cluster", "default", "Cluster name")
	f.StringVar(&cfg.ClusterToken, "cluster-token", "", "Shared cluster token")
	f.StringVar(&cfg.ContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "Containerd socket path")
	f.StringVar(&cfg.WireGuardPool, "wireguard-pool", "10.64.0.0/10", "Cluster address pool used for automatic namespace networking")
	f.StringVar(&cfg.WireGuardEndpoint, "wireguard-endpoint", "", "Externally reachable WireGuard host or host:port")
	f.IntVar(&cfg.WireGuardPort, "wireguard-port", 51820, "WireGuard UDP listen port")
	f.StringVar(&cfg.DNSListen, "dns-listen", ":8053", "DNS resolver listen address")
	f.StringVar(&cfg.CACert, "ca-cert", "", "Path to cluster CA certificate (PEM)")
	f.StringVar(&cfg.CAKey, "ca-key", "", "Path to cluster CA private key (PEM)")
	f.StringVar(&cfg.Cert, "cert", "", "Path to node certificate (PEM)")
	f.StringVar(&cfg.Key, "key", "", "Path to node private key (PEM)")
	f.StringArrayVar(&cfg.Labels, "label", nil, "Node label in key=value form (repeatable)")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
