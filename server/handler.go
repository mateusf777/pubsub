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

// DataProcessor decouples dataProcessor from ConnectionHandler.
type DataProcessor interface {
	Process()
}

// KeepAlive decouples core.KeepAlive from ConnectionHandler.
type KeepAlive interface {
	Run()
}

type ConnectionHandler struct {
	conn            ClientConn
	ps              PubSubConn
	cr              ConnReader
	dataProcessor   DataProcessor
	keepalive       KeepAlive
	data            chan []byte
	resetInactivity chan bool
	stopKeepAlive   chan bool
	closeHandler    chan bool
}

func NewConnectionHandler(conn ClientConn, ps PubSubConn) (*ConnectionHandler, error) {
	// Creates channels necessary for communication between the components running concurrently.
	dataCh := make(chan []byte)
	resetCh := make(chan bool)
	closeCh := make(chan bool)
	stopCh := make(chan bool)

	cr, err := core.NewConnectionReader(core.ConnectionReaderConfig{
		Reader:   conn,
		DataChan: dataCh,
	})
	if err != nil {
		slog.Error("NewConnectionHandler", "error", err)
		return nil, err
	}

	ka, err := core.NewKeepAlive(core.KeepAliveConfig{
		Writer:          conn,
		Client:          conn.RemoteAddr().String(),
		CloseHandler:    closeCh,
		ResetInactivity: resetCh,
		StopKeepAlive:   stopCh,
	})
	if err != nil {
		slog.Error("NewConnectionHandler", "error", err)
		return nil, err
	}

	return &ConnectionHandler{
		conn: conn,
		ps:   ps,
		cr:   cr,
		dataProcessor: &dataProcessor{
			conn:            conn,
			ps:              ps,
			data:            dataCh,
			resetInactivity: resetCh,
			stopKeepAlive:   stopCh,
			closeHandler:    closeCh,
		},
		keepalive:       ka,
		data:            dataCh,
		resetInactivity: resetCh,
		stopKeepAlive:   stopCh,
		closeHandler:    closeCh,
	}, nil
}

func (h *ConnectionHandler) Handle() {
	defer func(c ClientConn) {
		err := c.Close()
		if err != nil {
			slog.Error("Server.handleConnection", "error", err)
		}
		slog.Info("Closed connection", "remote", c.RemoteAddr().String())
	}(h.conn)

	go h.cr.Read()
	go h.dataProcessor.Process()
	go h.keepalive.Run()

	<-h.closeHandler
	h.ps.UnsubAll(h.conn.RemoteAddr().String())
}

type dataProcessor struct {
	conn            ClientConn
	ps              PubSubConn
	data            chan []byte
	resetInactivity chan bool
	stopKeepAlive   chan bool
	closeHandler    chan bool
}

// Process waits for data, verifies command and dispatches it to the appropriated handler, and writes the response to the connection.
func (d *dataProcessor) Process() {
	client := d.conn.RemoteAddr().String()

	for netData := range d.data {
		// If receive data from client, reset keep-alive
		d.resetInactivity <- true

		data := bytes.TrimSpace(netData)

		var result []byte
		switch {
		// STOP
		case bytes.Equal(bytes.ToUpper(data), core.OpStop), bytes.Equal(data, core.ControlC):
			slog.Info("Closing connection", "remote", client)
			// If client send STOP we close resources
			d.stopKeepAlive <- true
			d.closeHandler <- true
			break

		// PING
		case bytes.Equal(bytes.ToUpper(data), core.OpPing):
			result = core.BuildBytes(core.OpPong, core.CRLF)

		// PONG
		case bytes.Equal(bytes.ToUpper(data), core.OpPong):
			result = core.OK

		// PUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpPub):
			result = handlePub(d.ps, data, d.data)

		// SUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpSub):
			slog.Debug("sub", "value", data)
			result = handleSub(d.conn, d.ps, client, data)

		// UNSUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpUnsub):
			result = handleUnsub(d.ps, client, data)

		// NO DATA
		case bytes.Equal(data, core.Empty):
			continue

		default:
			// UNKNOWN
			result = core.BuildBytes([]byte("-ERR invalid protocol"), core.CRLF)
		}

		_, err := d.conn.Write(result)
		if err != nil {
			if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
				continue
			}
			slog.Error("server handler handleConnection", "error", err)
		}

	}
}

// handlePub Handles PUB message. See core.OpPub.
func handlePub(ps PubSubConn, received []byte, dataCh <-chan []byte) []byte {
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
	ps.Publish(string(args[1]), msg, opts...)

	return result
}

// handleUnsub Handles UNSUB. See core.OpUnsub.
func handleUnsub(ps PubSubConn, client string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse
	args := bytes.Split(received, core.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// Dispatch
	err := ps.Unsubscribe(string(args[1]), client, id)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}
	return result
}

// handleSub Handles SUB. See core.OpSub.
func handleSub(c ClientConn, ps PubSubConn, client string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse
	args := bytes.Split(received, core.Space)
	if len(args) < 3 || len(args) > 4 {
		return []byte("-ERR should be SUB <subject> <id> [group]\n")
	}

	id, _ := strconv.Atoi(string(args[2]))
	opts := make([]SubOpt, 0)

	// Optional group
	if len(args) == 4 {
		group := args[3]
		opts = append(opts, WithGroup(string(group)))
	}
	opts = append(opts, WithID(id))

	// Dispatch
	err := ps.Subscribe(string(args[1]), client, func(msg Message) {
		err := sendMsg(c, id, msg)
		if err != nil {
			slog.Error("send", "error", err)
		}
	}, opts...)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}

	return result
}

// sendMsg sends MSG back to client. See core.OpMsg.
func sendMsg(conn ClientConn, id int, msg Message) error {

	subID := strconv.AppendInt([]byte{}, int64(id), 10)
	result := core.BuildBytes(core.OpMsg, core.Space, []byte(msg.Subject), core.Space, subID, core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF)
	slog.Debug("MSG message", "result", result)

	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}

	return nil
}
