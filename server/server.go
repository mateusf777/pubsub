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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	l, err := net.Listen("tcp4", address)
	if err != nil {
		slog.Error("Server.Run", "error", err)
		return
	}
	defer func() {
		if err := l.Close(); err != nil {
			slog.Error("Server.Run", "error", err)
		}
	}()

	go acceptClients(l)

	slog.Info("PubSub accepting connections", "address", address)
	Wait()
	slog.Info("Stopping PubSub")
}

// acceptClients starts a concurrent handler for each connection
func acceptClients(l net.Listener) {
	// PubSub engine is unique per instance.
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

		ch, err := core.NewConnectionHandler(core.ConnectionHandlerConfig{
			Conn:       c,
			MsgHandler: MessageHandler(ps, c.RemoteAddr().String()),
		})
		if err != nil {
			slog.Error("NewConnectionHandler", "error", err)
			return
		}

		serverConnHandler := &ConnHandler{
			conn:        c,
			connHandler: ch,
			pubSub:      ps,
			remote:      c.RemoteAddr().String(),
		}

		if err := serverConnHandler.Connect(); err != nil {
			slog.Error("Connect", "error", err)
			return
		}
		go serverConnHandler.Run()
	}
}

// Wait for system signals (SIGINT, SIGTERM)
func Wait() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
