package main

import (
	"log/slog"
	"os"

	"github.com/mateusf777/pubsub/server"
)

// ATTENTION: This server supports TLS for secure transport and, if a CA is configured,
// will validate client certificates for authentication. If you run without TLS or CA,
// connections will not be encrypted or authenticated.
const (
	defaultAddress       = "0.0.0.0:9999"
	defaultHealthAddress = "0.0.0.0:8080"
)

func main() {
	address := os.Getenv("PUBSUB_ADDRESS")
	if len(address) == 0 {
		address = defaultAddress
	}

	certFile := os.Getenv("PUBSUB_TLS_CERT")
	keyFile := os.Getenv("PUBSUB_TLS_KEY")
	caFile := os.Getenv("PUBSUB_TLS_CA")

	// Health check endpoint configuration
	healthAddress := os.Getenv("PUBSUB_HEALTH_ADDRESS")
	if len(healthAddress) == 0 {
		healthAddress = defaultHealthAddress
	}
	enableHealth := os.Getenv("PUBSUB_ENABLE_HEALTH") != "false" // Enabled by default

	ps := server.NewPubSub(server.PubSubConfig{})

	// Start health check server if enabled
	var healthCheck *server.HealthCheck
	if enableHealth {
		tlsEnabled := certFile != "" && keyFile != ""
		healthCheck = server.NewHealthCheck(healthAddress, ps, tlsEnabled)
		go func() {
			slog.Info("Starting health check server", "address", healthAddress)
			if err := healthCheck.Start(); err != nil {
				slog.Error("Health check server error", "error", err)
			}
		}()
		// Mark as ready immediately for testing
		// In production, you might want to add additional readiness checks
		healthCheck.SetReady(true)

		// Cleanup on exit
		defer func() {
			if healthCheck != nil {
				healthCheck.SetReady(false)
				healthCheck.Close()
			}
		}()
	}

	if certFile != "" && keyFile != "" {
		server.Run(address, server.WithTLS(server.TLSConfig{
			CertFile: certFile,
			KeyFile:  keyFile,
			CAFile:   caFile,
		}), server.WithPubSub(ps))
	} else {
		server.Run(address, server.WithPubSub(ps))
	}
}
