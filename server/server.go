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

		cfg, err := buildConnHandlerConfig(c, ps)
		if err != nil {
			slog.Error("Server.acceptClient", "error", err)
			return
		}

		ch := NewConnectionHandler(cfg)
		go ch.Handle()
	}
}

// Wait for system signals (SIGINT, SIGTERM)
func Wait() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}

// buildConnHandlerConfig contains the ConnectionHandler configuration
// It puts together all the components:
// core.ConnectionReader, MessageProcessor, KeepAlive and communication channels.
func buildConnHandlerConfig(conn ClientConn, ps PubSubConn) (ConnectionHandlerConfig, error) {

	// Creates channels necessary for communication between the components running concurrently.
	dataCh := make(chan []byte)
	resetCh := make(chan bool)
	closeCh := make(chan bool)
	stopCh := make(chan bool)

	// Creates connection reader
	cr, err := core.NewConnectionReader(core.ConnectionReaderConfig{
		Reader:   conn,
		DataChan: dataCh,
	})
	if err != nil {
		slog.Error("NewConnectionHandler", "error", err)
		return ConnectionHandlerConfig{}, err
	}

	// Creates the keep alive
	ka, err := core.NewKeepAlive(core.KeepAliveConfig{
		Writer:          conn,
		Remote:          conn.RemoteAddr().String(),
		CloseHandler:    closeCh,
		ResetInactivity: resetCh,
		StopKeepAlive:   stopCh,
	})
	if err != nil {
		slog.Error("NewConnectionHandler", "error", err)
		return ConnectionHandlerConfig{}, err
	}

	// Creates the message processor
	mp := &messageProcessor{
		conn:            conn,
		pubSub:          ps,
		remote:          conn.RemoteAddr().String(),
		data:            dataCh,
		resetInactivity: resetCh,
		stopKeepAlive:   stopCh,
		closeHandler:    closeCh,
	}

	return ConnectionHandlerConfig{
		Conn:            conn,
		PubSub:          ps,
		ConnReader:      cr,
		MsgProc:         mp,
		KeepAlive:       ka,
		Data:            dataCh,
		ResetInactivity: resetCh,
		StopKeepAlive:   stopCh,
		CloseHandler:    closeCh,
	}, nil
}
