package server

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mateusf777/pubsub/core"
)

// TLSConfig holds the certificate and key file paths.
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// ServerOption is a functional option for server configuration.
type ServerOption func(*serverConfig)

type serverConfig struct {
	tlsConfig *TLSConfig
}

// WithTLS enables TLS using the provided certificate and key files.
func WithTLS(tlsConfig TLSConfig) ServerOption {
	return func(cfg *serverConfig) {
		cfg.tlsConfig = &tlsConfig
	}
}

// Run starts to listen in the given address, optionally with TLS.
// Pass WithTLS as an option to enable TLS.
func Run(address string, opts ...ServerOption) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	address = strings.TrimSpace(address)

	var l net.Listener
	var listenErr error

	if cfg.tlsConfig != nil {
		tlsCfg, err := loadTLSConfig(cfg.tlsConfig)
		if err != nil {
			slog.Error("Failed to load TLS configuration", "error", err)
			return
		}

		l, listenErr = tls.Listen("tcp4", address, tlsCfg)
	} else {
		l, listenErr = net.Listen("tcp4", address)
	}
	if listenErr != nil {
		slog.Error("Server.Run", "error", listenErr)
		return
	}
	defer func() {
		if err := l.Close(); err != nil {
			slog.Error("Server.Run", "error", err)
		}
	}()

	go acceptClients(l, cfg.tlsConfig != nil)

	slog.Info("PubSub accepting connections", "address", address, "tls", cfg.tlsConfig != nil)
	Wait()
	slog.Info("Stopping PubSub")
}

// acceptClients starts a concurrent handler for each connection
func acceptClients(l net.Listener, isTls bool) {
	// PubSub engine is unique per instance.
	ps := NewPubSub(PubSubConfig{})
	defer ps.Stop()

	for {
		c, err := l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), core.ClosedErr) {
				slog.Debug("Server.acceptClient (ClosedErr)", "error", err)
				return
			}
			slog.Error("Server.acceptClients", "error", err)
			return
		}

		if isTls {
			tlsConn, ok := c.(*tls.Conn)
			if !ok {
				slog.Error("Server.acceptClients", "error", "expected *tls.Conn")
				tlsConn.Close()
				continue
			}
			if err := tlsConn.Handshake(); err != nil {
				slog.Error("Server.acceptClients Handshake", "error", err)
				tlsConn.Close()
				continue
			}
		}

		ch, err := core.NewConnectionHandler(core.ConnectionHandlerConfig{
			Conn:       c,
			MsgHandler: MessageHandler(ps, c.RemoteAddr().String()),
		})
		if err != nil {
			slog.Error("NewConnectionHandler", "error", err)
			continue
		}

		serverConnHandler := &ConnHandler{
			conn:        c,
			connHandler: ch,
			pubSub:      ps,
			remote:      c.RemoteAddr().String(),
		}

		if err := serverConnHandler.Connect(); err != nil {
			slog.Error("Connect", "error", err)
			continue
		}

		slog.Info("New connection", "remote", serverConnHandler.remote)
		go serverConnHandler.Run()
	}
}

// Wait for system signals (SIGINT, SIGTERM)
func Wait() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}

func loadTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		slog.Error("Failed to load TLS certificate", "error", err)
		return nil, err
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}

	if cfg.CAFile != "" {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert

		caCertFile, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			slog.Error("Failed to read CA file", "error", err)
			return nil, err
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCertFile) {
			slog.Error("Failed to append CA certificate to pool", "error", err)
			return nil, err
		}
		tlsCfg.ClientCAs = caPool
	}
	return tlsCfg, nil
}
