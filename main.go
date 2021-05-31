package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const defaultAddress = "127.0.0.1:9999"

func main() {
	rand.Seed(time.Now().Unix())

	address := os.Getenv("PUBSUB_ADDRESS")
	if address == "" {
		address = defaultAddress
	}

	// Start listener
	l, err := net.Listen("tcp4", address)
	if err != nil {
		Log(ERR, "%v\n", err)
		return
	}
	defer l.Close()

	go AcceptClients(l)

	Log(INFO, "PubSub accepting connections at %s", address)
	Wait()
}

func Wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println()
	Log(INFO, "PubSub stop")
}

func AcceptClients(l net.Listener) {
	ps := NewPubSub()
	defer ps.Stop()

	for {
		c, err := l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), closeErr) {
				return
			}
			Log(ERR, "%v\n", err)
			return
		}

		go HandleConnection(c, ps)
	}
}
