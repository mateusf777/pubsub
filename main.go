package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/mateusf777/pubsub/domain"

	"github.com/mateusf777/pubsub/log"

	"github.com/mateusf777/pubsub/server"
)

// ATTENTION: This server has NO SECURITY whatsoever
// If you expose it you're on your own.
const defaultAddress = "127.0.0.1:9999"

func main() {
	rand.Seed(time.Now().Unix())

	address := os.Getenv("PUBSUB_ADDRESS")
	if address == string(domain.Empty) {
		address = defaultAddress
	}

	s := server.New(server.LogLevel(log.INFO))
	s.Run(address)
}
