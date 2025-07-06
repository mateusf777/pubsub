package main

import (
	"os"

	"github.com/mateusf777/pubsub/server"
)

// ATTENTION: This server has NO SECURITY whatsoever
// If you expose it you're on your own.
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
