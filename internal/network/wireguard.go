package network

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Peer struct {
	PublicKey  string   `json:"public_key"`
	Endpoint   string   `json:"endpoint"`
	AllowedIPs []string `json:"allowed_ips"`
}
type Config struct {
	CIDR             string `json:"cidr"`
	Gateway          string `json:"gateway"`
	WireGuardAddress string `json:"wireguard_address"`
	PrivateKeyFile   string `json:"private_key_file"`
	ListenPort       int    `json:"listen_port"`
	Peers            []Peer `json:"peers"`
}

type commandRunner interface {
	Run(context.Context, string, ...string) error
}
type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

type WireGuardManager struct {
	configDir string
	stateDir  string
	run       commandRunner
	mu        sync.Mutex
}

func NewWireGuardManager(configDir string) *WireGuardManager {
	return &WireGuardManager{configDir: configDir, stateDir: "/var/lib/trellis/network", run: execRunner{}}
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)
var safeAllocation = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,238}$`)

func (m *WireGuardManager) load(name string) (*Config, error) {
	if !safeName.MatchString(name) {
		return nil, fmt.Errorf("invalid network name %q", name)
	}
	raw, err := os.ReadFile(filepath.Join(m.configDir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf("read network config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse network config: %w", err)
	}
	prefix, err := netip.ParsePrefix(cfg.CIDR)
	if err != nil || !prefix.Addr().Is4() {
		return nil, fmt.Errorf("network CIDR must be IPv4: %q", cfg.CIDR)
	}
	if _, err := netip.ParseAddr(cfg.Gateway); err != nil {
		return nil, fmt.Errorf("invalid gateway: %w", err)
	}
	if gateway, _ := netip.ParseAddr(cfg.Gateway); !prefix.Contains(gateway) {
		return nil, fmt.Errorf("gateway must be within CIDR")
	}
	if _, err := netip.ParsePrefix(cfg.WireGuardAddress); err != nil {
		return nil, fmt.Errorf("invalid wireguard_address: %w", err)
	}
	if cfg.ListenPort < 1 || cfg.ListenPort > 65535 || cfg.PrivateKeyFile == "" {
		return nil, fmt.Errorf("private_key_file and valid listen_port are required")
	}
	keyInfo, err := os.Stat(cfg.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("stat private key: %w", err)
	}
	if keyInfo.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("private key file must not be accessible by group or others")
	}
	for _, peer := range cfg.Peers {
		if strings.TrimSpace(peer.PublicKey) == "" || len(peer.AllowedIPs) == 0 {
			return nil, fmt.Errorf("peer public_key and allowed_ips are required")
		}
		for _, allowed := range peer.AllowedIPs {
			if _, err := netip.ParsePrefix(allowed); err != nil {
				return nil, fmt.Errorf("invalid peer allowed IP %q: %w", allowed, err)
			}
		}
	}
	return &cfg, nil
}

func short(prefix, value string) string {
	h := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%s%x", prefix, h[:5])
}

func allocationAddress(cidr, allocation string) (string, error) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(allocation))
	host := binary.BigEndian.Uint32(h[:4])
	base := binary.BigEndian.Uint32(p.Addr().AsSlice())
	bits := uint32(32 - p.Bits())
	if bits < 3 {
		return "", fmt.Errorf("CIDR %s has no allocation space", cidr)
	}
	mask := uint32(1<<bits) - 1
	host = host%(mask-2) + 2
	a := netip.AddrFrom4([4]byte{byte(base >> 24), byte(base >> 16), byte(base >> 8), byte(base)}).Next()
	// Add host-1 without depending on platform integer address APIs.
	b := binary.BigEndian.Uint32(a.AsSlice()) + host - 1
	return fmt.Sprintf("%d.%d.%d.%d/%d", byte(b>>24), byte(b>>16), byte(b>>8), byte(b), p.Bits()), nil
}

func (m *WireGuardManager) Attach(ctx context.Context, tenant, networkName, allocation string) (_ *Attachment, retErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !safeName.MatchString(tenant) || !safeAllocation.MatchString(allocation) {
		return nil, fmt.Errorf("tenant and allocation must be safe identifiers")
	}
	cfg, err := m.load(networkName)
	if err != nil {
		return nil, err
	}
	wg, bridge, hostVeth, peerVeth := short("tw", tenant+"\x00"+networkName), short("tb", tenant+"\x00"+networkName), short("vh", allocation), short("vc", allocation)
	ns := filepath.Join("/var/run/netns", allocation)
	address, err := allocationAddress(cfg.CIDR, allocation)
	if err != nil {
		return nil, err
	}
	leaseDir := filepath.Join(m.stateDir, networkName)
	if err := os.MkdirAll(leaseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create IPAM state: %w", err)
	}
	lease := filepath.Join(leaseDir, strings.ReplaceAll(address, "/", "_"))
	f, err := os.OpenFile(lease, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("reserve address %s: %w", address, err)
	}
	_, _ = f.WriteString(allocation)
	_ = f.Close()
	defer func() {
		if retErr != nil {
			_ = os.Remove(lease)
		}
	}()
	// Every command is idempotently reconciled; "replace" is used for routes.
	_ = m.run.Run(ctx, "ip", "link", "add", bridge, "type", "bridge")
	_ = m.run.Run(ctx, "ip", "addr", "add", cfg.Gateway+"/"+strings.Split(cfg.CIDR, "/")[1], "dev", bridge)
	if err = m.run.Run(ctx, "ip", "link", "set", bridge, "up"); err != nil {
		return nil, err
	}
	_ = m.run.Run(ctx, "ip", "link", "add", wg, "type", "wireguard")
	_ = m.run.Run(ctx, "ip", "addr", "add", cfg.WireGuardAddress, "dev", wg)
	if err = m.run.Run(ctx, "wg", "set", wg, "private-key", cfg.PrivateKeyFile, "listen-port", fmt.Sprint(cfg.ListenPort)); err != nil {
		return nil, err
	}
	for _, p := range cfg.Peers {
		args := []string{"set", wg, "peer", p.PublicKey, "allowed-ips", strings.Join(p.AllowedIPs, ",")}
		if p.Endpoint != "" {
			args = append(args, "endpoint", p.Endpoint)
		}
		if err = m.run.Run(ctx, "wg", args...); err != nil {
			return nil, err
		}
		for _, route := range p.AllowedIPs {
			_ = m.run.Run(ctx, "ip", "route", "replace", route, "dev", wg)
		}
	}
	if err = m.run.Run(ctx, "ip", "link", "set", wg, "up"); err != nil {
		return nil, err
	}
	if err = m.run.Run(ctx, "sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return nil, err
	}
	if m.run.Run(ctx, "iptables", "-C", "FORWARD", "-i", bridge, "!", "-o", wg, "-j", "DROP") != nil {
		if err = m.run.Run(ctx, "iptables", "-A", "FORWARD", "-i", bridge, "!", "-o", wg, "-j", "DROP"); err != nil {
			return nil, err
		}
	}
	if m.run.Run(ctx, "iptables", "-C", "FORWARD", "-o", bridge, "!", "-i", wg, "-j", "DROP") != nil {
		if err = m.run.Run(ctx, "iptables", "-A", "FORWARD", "-o", bridge, "!", "-i", wg, "-j", "DROP"); err != nil {
			return nil, err
		}
	}
	if m.run.Run(ctx, "iptables", "-C", "INPUT", "-i", bridge, "!", "-d", cfg.Gateway, "-j", "DROP") != nil {
		if err = m.run.Run(ctx, "iptables", "-A", "INPUT", "-i", bridge, "!", "-d", cfg.Gateway, "-j", "DROP"); err != nil {
			return nil, err
		}
	}
	if err = m.run.Run(ctx, "ip", "netns", "add", allocation); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			_ = m.run.Run(ctx, "ip", "link", "del", hostVeth)
			_ = m.run.Run(ctx, "ip", "netns", "del", allocation)
		}
	}()
	if err = m.run.Run(ctx, "ip", "link", "add", hostVeth, "type", "veth", "peer", "name", peerVeth); err != nil {
		_ = m.run.Run(ctx, "ip", "netns", "del", allocation)
		return nil, err
	}
	_ = m.run.Run(ctx, "ip", "link", "set", hostVeth, "master", bridge)
	_ = m.run.Run(ctx, "ip", "link", "set", hostVeth, "up")
	if err = m.run.Run(ctx, "ip", "link", "set", peerVeth, "netns", allocation); err != nil {
		return nil, err
	}
	_ = m.run.Run(ctx, "ip", "-n", allocation, "link", "set", "lo", "up")
	_ = m.run.Run(ctx, "ip", "-n", allocation, "link", "set", peerVeth, "name", "eth0")
	if err = m.run.Run(ctx, "ip", "-n", allocation, "addr", "add", address, "dev", "eth0"); err != nil {
		return nil, err
	}
	_ = m.run.Run(ctx, "ip", "-n", allocation, "link", "set", "eth0", "up")
	if err = m.run.Run(ctx, "ip", "-n", allocation, "route", "replace", "default", "via", cfg.Gateway); err != nil {
		return nil, err
	}
	return &Attachment{AllocationID: allocation, Tenant: tenant, Network: networkName, Namespace: ns, HostVeth: hostVeth, Address: address, LeasePath: lease}, nil
}

func (m *WireGuardManager) Detach(ctx context.Context, a *Attachment) error {
	if a == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.run.Run(ctx, "ip", "link", "del", a.HostVeth)
	if err := m.run.Run(ctx, "ip", "netns", "del", a.AllocationID); err != nil {
		return err
	}
	if a.LeasePath != "" {
		if err := os.Remove(a.LeasePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
