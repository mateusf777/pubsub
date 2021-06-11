package main

import (
	"math/rand"
	"os"
	"time"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/log"

	"github.com/mateusf777/pubsub/server"
)

// ATTENTION: This server has NO SECURITY whatsoever
// If you expose it you're on your own.
const defaultAddress = "127.0.0.1:9999"

func main() {
	rand.Seed(time.Now().Unix())
	log.SetLevel(log.INFO)

	address := os.Getenv("PUBSUB_ADDRESS")
	if address == string(psnet.Empty) {
		address = defaultAddress
	}

	server.Run(address)
}
