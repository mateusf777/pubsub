package main

import (
	"os"

	"github.com/mateusf777/pubsub/server"
)

// ATTENTION: This server supports TLS for secure transport and, if a CA is configured,
// will validate client certificates for authentication. If you run without TLS or CA,
// connections will not be encrypted or authenticated.
const defaultAddress = "0.0.0.0:9999"

func main() {
	address := os.Getenv("PUBSUB_ADDRESS")
	if len(address) == 0 {
		address = defaultAddress
	}

	certFile := os.Getenv("PUBSUB_TLS_CERT")
	keyFile := os.Getenv("PUBSUB_TLS_KEY")
	caFile := os.Getenv("PUBSUB_TLS_CA")

	ps := server.NewPubSub(server.PubSubConfig{})

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
