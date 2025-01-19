package server

import (
	"log/slog"
	"net"
	"strings"

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
	core.Wait()
	slog.Info("Stopping PubSub")
}

// Starts a concurrent handler for each connection
func acceptClients(l net.Listener) {
	ps := core.NewPubSub()
	defer ps.Stop()

	for {
		c, err := l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), core.CloseErr) {
				slog.Error("Server.acceptClient (CloseErr)", "error", err)
				return
			}
			slog.Error("Server.acceptClients", "error", err)
			return
		}

		go handleConnection(c, ps)
	}
}
