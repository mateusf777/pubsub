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

	server.Run(address)
}
