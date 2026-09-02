package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type nodeConfigFile struct {
	AgentListen       *string   `yaml:"agent_listen"`
	AgentAdvertise    *string   `yaml:"agent_advertise"`
	ServerListen      *string   `yaml:"server_listen"`
	ServerAdvertise   *string   `yaml:"server_advertise"`
	RaftListen        *string   `yaml:"raft_listen"`
	RaftAdvertise     *string   `yaml:"raft_advertise"`
	Join              *string   `yaml:"join"`
	DataDir           *string   `yaml:"data_dir"`
	Cluster           *string   `yaml:"cluster"`
	BootstrapToken    *string   `yaml:"bootstrap_token"`
	ContainerdSock    *string   `yaml:"containerd_socket"`
	Runtime           *string   `yaml:"runtime"`
	RuntimeFaults     *string   `yaml:"runtime_faults"`
	WireGuardPool     *string   `yaml:"wireguard_pool"`
	WireGuardEndpoint *string   `yaml:"wireguard_endpoint"`
	WireGuardPort     *int      `yaml:"wireguard_port"`
	DNSListen         *string   `yaml:"dns_listen"`
	CACert            *string   `yaml:"ca_cert"`
	CAKey             *string   `yaml:"ca_key"`
	Cert              *string   `yaml:"cert"`
	Key               *string   `yaml:"key"`
	SecretsKey        *string   `yaml:"secrets_key"`
	SecretsKeyID      *string   `yaml:"secrets_key_id"`
	Labels            *[]string `yaml:"labels"`
	HostVolumes       *[]string `yaml:"host_volumes"`
}

func loadNodeConfig(path string, cfg *config, flags *pflag.FlagSet) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	var parsed nodeConfigFile
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}

	setString := func(flag string, value *string, target *string) {
		if value != nil && !flags.Changed(flag) {
			*target = *value
		}
	}
	setString("agent-listen", parsed.AgentListen, &cfg.AgentListen)
	setString("agent-advertise", parsed.AgentAdvertise, &cfg.AgentAdvertise)
	setString("server-listen", parsed.ServerListen, &cfg.ServerListen)
	setString("server-advertise", parsed.ServerAdvertise, &cfg.ServerAdvertise)
	setString("raft-listen", parsed.RaftListen, &cfg.RaftListen)
	setString("raft-advertise", parsed.RaftAdvertise, &cfg.RaftAdvertise)
	setString("join", parsed.Join, &cfg.Join)
	setString("data-dir", parsed.DataDir, &cfg.DataDir)
	setString("cluster", parsed.Cluster, &cfg.Cluster)
	setString("bootstrap-token", parsed.BootstrapToken, &cfg.BootstrapToken)
	setString("containerd-sock", parsed.ContainerdSock, &cfg.ContainerdSock)
	setString("runtime", parsed.Runtime, &cfg.Runtime)
	setString("runtime-faults", parsed.RuntimeFaults, &cfg.RuntimeFaults)
	setString("wireguard-pool", parsed.WireGuardPool, &cfg.WireGuardPool)
	setString("wireguard-endpoint", parsed.WireGuardEndpoint, &cfg.WireGuardEndpoint)
	setString("dns-listen", parsed.DNSListen, &cfg.DNSListen)
	setString("ca-cert", parsed.CACert, &cfg.CACert)
	setString("ca-key", parsed.CAKey, &cfg.CAKey)
	setString("cert", parsed.Cert, &cfg.Cert)
	setString("key", parsed.Key, &cfg.Key)
	setString("secrets-key", parsed.SecretsKey, &cfg.SecretsKey)
	setString("secrets-key-id", parsed.SecretsKeyID, &cfg.SecretsKeyID)
	if parsed.WireGuardPort != nil && !flags.Changed("wireguard-port") {
		cfg.WireGuardPort = *parsed.WireGuardPort
	}
	if parsed.Labels != nil && !flags.Changed("label") {
		cfg.Labels = append([]string(nil), (*parsed.Labels)...)
	}
	if parsed.HostVolumes != nil && !flags.Changed("host-volume") {
		cfg.HostVolumes = append([]string(nil), (*parsed.HostVolumes)...)
	}
	return nil
}
