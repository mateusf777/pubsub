package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

// ClientConn wraps network client connection.
type ClientConn interface {
	net.Conn
}

// PubSubConn provides an API to the PubSub engine.
type PubSubConn interface {
	// Stop closes the message channel, stopping the engine.
	Stop()
	// Publish message to subject handlers.
	Publish(subject string, data []byte, opts ...PubOpt)
	// Subscribe a client handler to a subject.
	Subscribe(subject string, client string, handler Handler, opts ...SubOpt) error
	// Unsubscribe a client handler from a subject.
	Unsubscribe(subject string, client string, id int) error
	// UnsubAll client handlers.
	UnsubAll(client string)
	// run the engine.
	run()
	// hasSubscriber verifies the existence of at least one subscriber to a subject.
	hasSubscriber(subject string) bool
}

// ConnReader decouples core.ConnectionReader from ConnectionHandler.
type ConnReader interface {
	Read()
}

// MessageProcessor decouples messageProcessor from ConnectionHandler.
type MessageProcessor interface {
	Process()
}

// KeepAlive decouples core.KeepAlive from ConnectionHandler.
type KeepAlive interface {
	Run()
}

// ConnectionHandlerConfig it's used to create a ConnectionHandler.
type ConnectionHandlerConfig struct {
	Conn            ClientConn
	PubSub          PubSubConn
	ConnReader      ConnReader
	MsgProc         MessageProcessor
	KeepAlive       KeepAlive
	Data            chan []byte
	ResetInactivity chan bool
	StopKeepAlive   chan bool
	CloseHandler    chan bool
}

// ConnectionHandler handles an accepted client connection.
type ConnectionHandler struct {
	conn            ClientConn
	pubSub          PubSubConn
	connReader      ConnReader
	msgProc         MessageProcessor
	keepAlive       KeepAlive
	data            chan []byte
	resetInactivity chan bool
	stopKeepAlive   chan bool
	closeHandler    chan bool
}

// NewConnectionHandler creates all the necessary channels and components used by the connection handler.
func NewConnectionHandler(cfg ConnectionHandlerConfig) *ConnectionHandler {
	// Binds everything in the ConnectionHandler
	return &ConnectionHandler{
		conn:            cfg.Conn,
		pubSub:          cfg.PubSub,
		connReader:      cfg.ConnReader,
		msgProc:         cfg.MsgProc,
		keepAlive:       cfg.KeepAlive,
		data:            cfg.Data,
		resetInactivity: cfg.ResetInactivity,
		stopKeepAlive:   cfg.StopKeepAlive,
		closeHandler:    cfg.CloseHandler,
	}
}

// Handle the client connection
func (h *ConnectionHandler) Handle() {
	// Cleanup resources
	defer h.Close()

	// Reads messages from the connection
	go h.connReader.Read()
	// Process the messages
	go h.msgProc.Process()
	// Runs process that verifies connection activity
	go h.keepAlive.Run()

	// Waits for a signal to close the connection handler
	<-h.closeHandler
}

// Close cleanup ConnectionHandler resources
func (h *ConnectionHandler) Close() {
	// Removes all client subscribers from pubSub engine.
	h.pubSub.UnsubAll(h.conn.RemoteAddr().String())

	// Close all channels
	close(h.data)
	close(h.stopKeepAlive)
	close(h.resetInactivity)
	close(h.closeHandler)

	// Close the connection with the client
	err := h.conn.Close()
	if err != nil {
		slog.Error("Server.handleConnection", "error", err)
	}

	slog.Info("Closed connection", "remote", h.conn.RemoteAddr().String())
}

type messageProcessor struct {
	conn            ClientConn
	pubSub          PubSubConn
	data            chan []byte
	resetInactivity chan bool
	stopKeepAlive   chan bool
	closeHandler    chan bool
}

// Process waits for a client message, verifies and dispatches it to the appropriated handler, and writes the response to the connection.
func (m *messageProcessor) Process() {
	client := m.conn.RemoteAddr().String()

loop:
	for netData := range m.data {
		// If receive data from client, reset keep-alive
		m.resetInactivity <- true

		data := bytes.TrimSpace(netData)

		var result []byte
		switch {
		// STOP
		case bytes.Equal(bytes.ToUpper(data), core.OpStop), bytes.Equal(data, core.ControlC):
			slog.Info("Closing connection", "remote", client)
			// If client send STOP we close resources
			m.stopKeepAlive <- true
			m.closeHandler <- true
			break loop

		// PING
		case bytes.Equal(bytes.ToUpper(data), core.OpPing):
			result = core.BuildBytes(core.OpPong, core.CRLF)

		// PONG
		case bytes.Equal(bytes.ToUpper(data), core.OpPong):
			result = core.OK

		// PUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpPub):
			result = handlePub(m.pubSub, data, m.data)

		// SUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpSub):
			slog.Debug("sub", "value", data)
			result = handleSub(m.conn, m.pubSub, client, data)

		// UNSUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpUnsub):
			result = handleUnsub(m.pubSub, client, data)

		// NO DATA
		case bytes.Equal(data, core.Empty):
			continue

		default:
			// UNKNOWN
			result = core.BuildBytes([]byte("-ERR invalid protocol"), core.CRLF)
		}

		_, err := m.conn.Write(result)
		if err != nil {
			if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
				continue
			}
			slog.Error("server handler handleConnection", "error", err)
		}

	}
}

// handlePub Handles PUB message. See core.OpPub.
func handlePub(pubSub PubSubConn, received []byte, dataCh <-chan []byte) []byte {
	// Default result
	result := core.OK

	// Get next line
	msg := <-dataCh

	// Parse
	args := bytes.Split(received, core.Space)
	if len(args) < 2 || len(args) > 3 {
		return []byte("-ERR should be PUB <subject> [reply-to]   \n")
	}

	opts := make([]PubOpt, 0)

	// Optional Reply
	if len(args) == 3 {
		reply := args[2]
		opts = append(opts, WithReply(string(reply)))
	}

	// Dispatch
	pubSub.Publish(string(args[1]), msg, opts...)

	return result
}

// handleUnsub Handles UNSUB. See core.OpUnsub.
func handleUnsub(pubSub PubSubConn, client string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse
	args := bytes.Split(received, core.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// Dispatch
	err := pubSub.Unsubscribe(string(args[1]), client, id)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}
	return result
}

// handleSub Handles SUB. See core.OpSub.
func handleSub(conn ClientConn, pubSub PubSubConn, client string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse
	args := bytes.Split(received, core.Space)
	if len(args) < 3 || len(args) > 4 {
		return []byte("-ERR should be SUB <subject> <id> [group]\n")
	}

	subID, _ := strconv.Atoi(string(args[2]))
	opts := make([]SubOpt, 0)

	// Optional group
	if len(args) == 4 {
		group := args[3]
		opts = append(opts, WithGroup(string(group)))
	}
	opts = append(opts, WithID(subID))

	// Dispatch
	err := pubSub.Subscribe(string(args[1]), client, subscriberHandler(conn, subID), opts...)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}

	return result
}

func subscriberHandler(conn ClientConn, sid int) Handler {
	return func(msg Message) {
		err := sendMsg(conn, sid, msg)
		if err != nil {
			slog.Error("send", "error", err)
		}
	}
}

// sendMsg sends MSG back to client. See core.OpMsg.
func sendMsg(conn ClientConn, sid int, msg Message) error {

	subID := strconv.AppendInt([]byte{}, int64(sid), 10)
	result := core.BuildBytes(core.OpMsg, core.Space, []byte(msg.Subject), core.Space, subID, core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF)
	slog.Debug("MSG message", "result", result)

	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}

	return nil
}
