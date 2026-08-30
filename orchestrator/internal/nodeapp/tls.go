package nodeapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/clofour/trellis/internal/storage"
	"github.com/clofour/trellis/internal/tlsutil"
)

func loadOrBootstrapTLS(ctx context.Context, log *slog.Logger, cfg *Config, local *storage.LocalStorage) (*tlsutil.Materials, error) {
	if cfg.CACert != "" {
		return loadTLSFromFiles(cfg)
	}
	materials, err := loadTLSFromStorage(local)
	if err == nil {
		return materials, nil
	}
	if cfg.Join != "" {
		response, err := joinClusterTLS(ctx, log, cfg.Join, cfg.ClusterToken, cfg.ServerAdvertise, cfg.RaftAdvertise)
		if err != nil {
			return nil, fmt.Errorf("join cluster for TLS: %w", err)
		}
		caCert, caKey := []byte(response.CACert), []byte(response.CAKey)
		nodeCert, nodeKey, err := tlsutil.GenerateNodeCert(caCert, caKey)
		if err != nil {
			return nil, fmt.Errorf("generate node cert: %w", err)
		}
		materials := &tlsutil.Materials{CACert: caCert, CAKey: caKey, Cert: nodeCert, Key: nodeKey}
		if err := saveTLSToStorage(local, materials); err != nil {
			return nil, fmt.Errorf("save TLS materials: %w", err)
		}
		log.Info("TLS materials received from cluster and stored")
		return materials, nil
	}
	caCert, caKey, err := tlsutil.GenerateCA()
	if err != nil {
		return nil, fmt.Errorf("generate CA: %w", err)
	}
	nodeCert, nodeKey, err := tlsutil.GenerateNodeCert(caCert, caKey)
	if err != nil {
		return nil, fmt.Errorf("generate node cert: %w", err)
	}
	materials = &tlsutil.Materials{CACert: caCert, CAKey: caKey, Cert: nodeCert, Key: nodeKey}
	if err := saveTLSToStorage(local, materials); err != nil {
		return nil, fmt.Errorf("save TLS materials: %w", err)
	}
	log.Info("cluster CA and node certificate generated")
	return materials, nil
}

func loadTLSFromFiles(cfg *Config) (*tlsutil.Materials, error) {
	caCert, err := os.ReadFile(cfg.CACert)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	caKey, err := os.ReadFile(cfg.CAKey)
	if err != nil {
		return nil, fmt.Errorf("read CA key: %w", err)
	}
	cert, err := os.ReadFile(cfg.Cert)
	if err != nil {
		return nil, fmt.Errorf("read cert: %w", err)
	}
	key, err := os.ReadFile(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}
	return &tlsutil.Materials{CACert: caCert, CAKey: caKey, Cert: cert, Key: key}, nil
}

func loadTLSFromStorage(local *storage.LocalStorage) (*tlsutil.Materials, error) {
	var caCert, caKey, cert, key string
	if err := local.Get("tls/ca-cert", &caCert); err != nil {
		return nil, err
	}
	if err := local.Get("tls/ca-key", &caKey); err != nil {
		return nil, err
	}
	if err := local.Get("tls/node-cert", &cert); err != nil {
		return nil, err
	}
	if err := local.Get("tls/node-key", &key); err != nil {
		return nil, err
	}
	return &tlsutil.Materials{CACert: []byte(caCert), CAKey: []byte(caKey), Cert: []byte(cert), Key: []byte(key)}, nil
}

func saveTLSToStorage(local *storage.LocalStorage, materials *tlsutil.Materials) error {
	if err := local.Put("tls/ca-cert", string(materials.CACert)); err != nil {
		return err
	}
	if err := local.Put("tls/ca-key", string(materials.CAKey)); err != nil {
		return err
	}
	if err := local.Put("tls/node-cert", string(materials.Cert)); err != nil {
		return err
	}
	return local.Put("tls/node-key", string(materials.Key))
}
