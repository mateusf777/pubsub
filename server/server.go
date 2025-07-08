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

// TLSConfig holds the certificate, key, and optional CA file paths for TLS setup.
// Used to configure secure transport and client certificate validation.
type TLSConfig struct {
	CertFile string // Path to the TLS certificate file.
	KeyFile  string // Path to the TLS private key file.
	CAFile   string // Path to the CA certificate file for client certificate validation (optional).
}

// ServerOption is a functional option for server configuration.
// Used to customize server behavior (e.g., enabling TLS, injecting a PubSub engine).
type ServerOption func(*serverConfig)

// serverConfig holds the configuration for the server, including TLS and PubSub engine.
type serverConfig struct {
	tlsConfig *TLSConfig // TLS configuration (if enabled).
	pubsub    *PubSub    // PubSub engine instance.
}

// WithTLS enables TLS using the provided certificate and key files.
// Optionally configures client certificate validation if CAFile is set.
func WithTLS(tlsConfig TLSConfig) ServerOption {
	return func(cfg *serverConfig) {
		cfg.tlsConfig = &tlsConfig
	}
}

// WithPubSub injects a custom PubSub engine into the server configuration.
func WithPubSub(pubsub *PubSub) ServerOption {
	return func(cfg *serverConfig) {
		cfg.pubsub = pubsub
	}
}

// Run starts the server, listening on the given address with optional TLS.
// Pass WithTLS as an option to enable TLS. Handles graceful shutdown on SIGINT/SIGTERM.
func Run(address string, opts ...ServerOption) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.pubsub == nil {
		cfg.pubsub = NewPubSub(PubSubConfig{})
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

	lh := listenerHandler{l, cfg.pubsub, cfg.tlsConfig != nil}
	go lh.handle()

	slog.Info("PubSub accepting connections", "address", address, "tls", cfg.tlsConfig != nil)
	Wait()
	slog.Info("Stopping PubSub")
}

// listenerHandler manages the main listener and handles incoming connections.
// For each accepted connection, it initializes the connection handler and PubSub engine.
type listenerHandler struct {
	listener net.Listener // The network listener (TCP or TLS).
	ps       *PubSub      // PubSub engine instance.
	isTls    bool         // Indicates if TLS is enabled.
}

// handle accepts new connections and starts a handler for each connection.
// Handles both plain TCP and TLS connections.
func (lh *listenerHandler) handle() {
	for {
		c, err := lh.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), core.ClosedErr) {
				slog.Debug("Server.acceptClient (ClosedErr)", "error", err)
				return
			}
			slog.Error("Server.acceptClients", "error", err)
			return
		}

		initilizeConnectionHanlder(c, lh.ps, lh.isTls)
	}
}

// initilizeConnectionHanlder initializes a new connection handler for the accepted connection.
// Performs TLS handshake and extracts tenant information if TLS is enabled.
func initilizeConnectionHanlder(c net.Conn, ps *PubSub, isTls bool) {
	var tenant string

	if isTls {
		tlsConn, ok := c.(*tls.Conn)
		if !ok {
			slog.Error("Server.acceptClients", "error", "expected *tls.Conn")
			tlsConn.Close()
			return
		}
		if err := tlsConn.Handshake(); err != nil {
			slog.Error("Server.acceptClients Handshake", "error", err)
			tlsConn.Close()
			return
		}

		tlsConnState := tlsConn.ConnectionState()
		if len(tlsConnState.PeerCertificates) > 0 {
			tenant = tlsConnState.PeerCertificates[0].SerialNumber.String()
			slog.Info("Server.acceptClients", "remote", c.RemoteAddr().String(), "tenant", tenant)
		} else {
			slog.Warn("Server.acceptClients, verify if you need to configure the server with CA cert to avoid this", "error", "no peer certificates found in TLS connection")
		}
	}

	ch, err := core.NewConnectionHandler(core.ConnectionHandlerConfig{
		Conn:       c,
		MsgHandler: MessageHandler(ps, tenant, c.RemoteAddr().String()),
	})
	if err != nil {
		slog.Error("NewConnectionHandler", "error", err)
		return
	}

	serverConnHandler := &ConnHandler{
		conn:        c,
		connHandler: ch,
		pubSub:      ps,
		remote:      c.RemoteAddr().String(),
	}

	if err := serverConnHandler.Connect(); err != nil {
		slog.Error("Connect", "error", err)
		return
	}

	slog.Info("New connection", "remote", serverConnHandler.remote)
	go serverConnHandler.Run()
}

// Wait blocks until a system signal (SIGINT or SIGTERM) is received.
// Used for graceful shutdown of the server.
func Wait() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}

// loadTLSConfig loads and configures the TLS settings for the server.
// Loads the certificate and key, and if a CA is provided, enables client certificate validation (mTLS).
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
		// Attempt to append the CA certificate to the pool for client certificate validation.
		// This enables mutual TLS (mTLS) if a CA is configured.
		if !caPool.AppendCertsFromPEM(caCertFile) {
			slog.Error("Failed to append CA certificate to pool", "error", err)
			return nil, err
		}
		tlsCfg.ClientCAs = caPool // Set the CA pool for client certificate verification.
	}
	return tlsCfg, nil
}
