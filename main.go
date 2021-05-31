package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/mateusf777/tcplab/server"
)

const defaultAddress = "127.0.0.1:9999"

func main() {
	rand.Seed(time.Now().Unix())

	address := os.Getenv("PUBSUB_ADDRESS")
	if address == "" {
		address = defaultAddress
	}

	server.Start(address)
}
