package server

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mateusf777/pubsub/core"
)

// Run stars to listen in the given address
// Ex: server.Run("localhost:9999")
func Run(address string) {
	l, err := net.Listen("tcp4", address)
	if err != nil {
		slog.Error("Server.Run", "error", err)
		return
	}
	defer l.Close()

	go acceptClients(l)

	slog.Info("PubSub accepting connections", "address", address)
	Wait()
	slog.Info("Stopping PubSub")
}

// Starts a concurrent handler for each connection
func acceptClients(l net.Listener) {
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

		go handleConnection(c, ps)
	}
}

// Wait for system signals (SIGINT, SIGTERM)
func Wait() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
