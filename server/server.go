package server

import (
	"log/slog"
	"net"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

type Server struct{}

// Run stars to listen in the given address
// Ex: server.Run("localhost:9999")
func (s Server) Run(address string) {
	l, err := net.Listen("tcp4", address)
	if err != nil {
		slog.Error("Server.Run", "error", err)
		return
	}
	defer l.Close()

	go s.acceptClients(l)

	slog.Info("PubSub accepting connections", "address", address)
	core.Wait()
	slog.Info("Stopping PubSub")
}

// Starts a concurrent handler for each connection
func (s Server) acceptClients(l net.Listener) {
	ps := core.NewPubSub()
	defer ps.Stop()

	for {
		c, err := l.Accept()
		if err != nil {
			// Todo: better handle connection to stop hiding errors like this
			if strings.Contains(err.Error(), core.CloseErr) {
				return
			}
			slog.Error("Server.acceptClients", "error", err)
			return
		}

		go s.handleConnection(c, ps)
	}
}
