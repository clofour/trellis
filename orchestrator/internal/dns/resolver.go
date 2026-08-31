// Package dns provides DNS-based Trellis service discovery.
package dns

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
)

const (
	// DefaultDomain is the default DNS suffix for Trellis services.
	DefaultDomain = "trellis"
	// DefaultTTL is the default lifetime of DNS answers, in seconds.
	DefaultTTL = 5
	maxUDPSize = 512
)

// DiscoveryLookup lists service-discovery records.
type DiscoveryLookup interface {
	ListDiscovery(ctx context.Context) (*api.ServiceListResponse, error)
}

type record struct {
	addresses []net.IP
}

// Resolver serves DNS records backed by service discovery.
type Resolver struct {
	log    *slog.Logger
	domain string
	lookup DiscoveryLookup

	mu    sync.RWMutex
	cache map[string]*record // "job.namespace" -> record
}

// NewResolver creates a DNS resolver for the supplied discovery source.
func NewResolver(log *slog.Logger, lookup DiscoveryLookup, domain string) *Resolver {
	if domain == "" {
		domain = DefaultDomain
	}
	return &Resolver{
		log:    log,
		domain: domain,
		lookup: lookup,
		cache:  make(map[string]*record),
	}
}

// Run serves DNS queries on addr until ctx is canceled.
func (r *Resolver) Run(ctx context.Context, addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve listen address: %w", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	defer func() { _ = conn.Close() }()

	go r.refreshLoop(ctx)

	r.log.Info("dns resolver started", "addr", addr, "domain", r.domain)

	buf := make([]byte, maxUDPSize)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return fmt.Errorf("set DNS read deadline: %w", err)
		}
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			r.log.Error("dns read error", "error", err)
			continue
		}
		response := r.handleQuery(buf[:n])
		if response != nil {
			if _, err := conn.WriteToUDP(response, remote); err != nil {
				r.log.Error("dns write error", "error", err)
			}
		}
	}
}

func (r *Resolver) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	r.refresh(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refresh(ctx)
		}
	}
}

func (r *Resolver) refresh(ctx context.Context) {
	resp, err := r.lookup.ListDiscovery(ctx)
	if err != nil {
		r.log.Error("dns refresh failed", "error", err)
		return
	}
	if resp == nil {
		return
	}
	cache := make(map[string]*record)
	for _, svc := range *resp {
		if svc.Address == "" {
			continue
		}
		ip := net.ParseIP(svc.Address)
		if ip == nil {
			ips, err := net.LookupIP(svc.Address)
			if err != nil || len(ips) == 0 {
				continue
			}
			ip = ips[0]
		}
		key := svc.Job + "." + svc.Namespace
		rec, ok := cache[key]
		if !ok {
			rec = &record{}
			cache[key] = rec
		}
		rec.addresses = append(rec.addresses, ip)
	}
	r.mu.Lock()
	r.cache = cache
	r.mu.Unlock()
}

func (r *Resolver) resolve(name string) []net.IP {
	suffix := "." + r.domain + "."
	if !strings.HasSuffix(name, suffix) {
		return nil
	}
	query := strings.TrimSuffix(name, suffix)
	parts := strings.SplitN(query, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	job, namespace := parts[0], parts[1]
	key := job + "." + namespace

	r.mu.RLock()
	rec := r.cache[key]
	r.mu.RUnlock()
	if rec == nil {
		return nil
	}
	return rec.addresses
}

// handleQuery parses a minimal DNS query and produces a response.
// Only A record queries (type 1, class IN) are answered.
func (r *Resolver) handleQuery(packet []byte) []byte {
	if len(packet) < 12 {
		return nil
	}

	id := binary.BigEndian.Uint16(packet[0:2])
	flags := binary.BigEndian.Uint16(packet[2:4])
	if flags&0x8000 != 0 {
		return nil // not a query
	}
	qdCount := binary.BigEndian.Uint16(packet[4:6])
	if qdCount == 0 {
		return nil
	}

	name, offset := decodeName(packet, 12)
	if offset < 0 || offset+4 > len(packet) {
		return nil
	}
	qtype := binary.BigEndian.Uint16(packet[offset : offset+2])
	qclass := binary.BigEndian.Uint16(packet[offset+2 : offset+4])

	if qtype != 1 || qclass != 1 {
		return buildResponse(id, name, qtype, qclass, nil)
	}

	ips := r.resolve(name)
	return buildResponse(id, name, qtype, qclass, ips)
}

func buildResponse(id uint16, name string, qtype, qclass uint16, ips []net.IP) []byte {
	buf := make([]byte, 0, maxUDPSize)

	// Header
	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:2], id)
	flags := uint16(0x8000) // QR=1 (response)
	flags |= 0x0400         // AA=1 (authoritative)
	if len(ips) == 0 {
		flags |= 0x0003 // RCODE=NXDOMAIN
	}
	binary.BigEndian.PutUint16(header[2:4], flags)
	binary.BigEndian.PutUint16(header[4:6], 1)                // QDCOUNT
	binary.BigEndian.PutUint16(header[6:8], uint16(len(ips))) // ANCOUNT
	buf = append(buf, header...)

	// Question section
	buf = append(buf, encodeName(name)...)
	qtypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint16(qtypeBytes[0:2], qtype)
	binary.BigEndian.PutUint16(qtypeBytes[2:4], qclass)
	buf = append(buf, qtypeBytes...)

	// Answer section
	for _, ip := range ips {
		ipv4 := ip.To4()
		if ipv4 == nil {
			continue
		}
		// Name pointer to offset 12 (start of question name)
		buf = append(buf, 0xC0, 0x0C)
		ans := make([]byte, 10)
		binary.BigEndian.PutUint16(ans[0:2], 1)          // TYPE A
		binary.BigEndian.PutUint16(ans[2:4], 1)          // CLASS IN
		binary.BigEndian.PutUint32(ans[4:8], DefaultTTL) // TTL
		binary.BigEndian.PutUint16(ans[8:10], 4)         // RDLENGTH
		buf = append(buf, ans...)
		buf = append(buf, ipv4...)
	}

	return buf
}

func encodeName(name string) []byte {
	name = strings.TrimSuffix(name, ".")
	var buf []byte
	for _, label := range strings.Split(name, ".") {
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0)
	return buf
}

func decodeName(packet []byte, offset int) (string, int) {
	var labels []string
	visited := make(map[int]bool)
	origOffset := -1
	for offset < len(packet) {
		if visited[offset] {
			return "", -1 // loop
		}
		visited[offset] = true
		length := int(packet[offset])
		if length == 0 {
			offset++
			break
		}
		if length&0xC0 == 0xC0 {
			if offset+1 >= len(packet) {
				return "", -1
			}
			ptr := int(binary.BigEndian.Uint16(packet[offset:offset+2])) & 0x3FFF
			if origOffset < 0 {
				origOffset = offset + 2
			}
			offset = ptr
			continue
		}
		offset++
		if offset+length > len(packet) {
			return "", -1
		}
		labels = append(labels, string(packet[offset:offset+length]))
		offset += length
	}
	if origOffset >= 0 {
		offset = origOffset
	}
	return strings.Join(labels, ".") + ".", offset
}
