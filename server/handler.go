package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

// PubSubConn provides an API to the PubSub engine.
// This interface abstracts the main operations for publishing, subscribing, and managing message handlers.
type PubSubConn interface {
	// Stop closes the message channel, stopping the engine.
	Stop()
	// Publish message to subject handlers.
	Publish(subject string, data []byte, opts ...PubOpt)
	// Subscribe a remote handler to a subject.
	Subscribe(subject string, client string, handler Handler, opts ...SubOpt) error
	// Unsubscribe a remote handler from a subject.
	Unsubscribe(subject string, client string, id int) error
	// UnsubAll remote handlers.
	UnsubAll(client string)
	// run the engine.
	run()
	// hasSubscriber verifies the existence of at least one subscriber to a subject.
	hasSubscriber(subject string) bool
}

// ConnectionHandler abstracts the lifecycle of a network connection for the pubsub server.
// It provides methods to handle and close the connection.
type ConnectionHandler interface {
	Handle()
	Close()
}

// ConnHandler manages a client/server connection and its associated pubsub engine.
// It coordinates the connection handler and pubsub engine for a remote client.
type ConnHandler struct {
	conn        net.Conn          // The underlying network connection.
	connHandler ConnectionHandler // Handler for connection lifecycle.
	pubSub      PubSubConn        // PubSub engine instance.
	remote      string            // Remote address string.
}

// Connect performs the initial handshake with the client, sending Info with client ID and nonce.
// Returns an error if the handshake fails.
func (s *ConnHandler) Connect() error {

	newSha := sha256.New()
	if _, err := newSha.Write([]byte(s.conn.RemoteAddr().String())); err != nil {
		return err
	}
	hash := newSha.Sum(nil)

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		slog.Error("NewConnectionHandler", "error", err)
		return err
	}

	info := core.Info{
		ClientID: base64.RawStdEncoding.EncodeToString(hash),
		Nonce:    base64.RawStdEncoding.EncodeToString(nonceBytes),
	}

	infoB, err := json.Marshal(info)
	if err != nil {
		return err
	}

	if _, err := s.conn.Write(core.BuildBytes(core.OpInfo, core.Space, infoB, core.CRLF)); err != nil {
		return err
	}

	return nil
}

// Run starts the connection handler and ensures resources are cleaned up on exit.
func (s *ConnHandler) Run() {
	defer s.Close()
	s.connHandler.Handle()
}

// Close unsubscribes all handlers for this remote and closes the connection handler.
func (s *ConnHandler) Close() {
	s.pubSub.UnsubAll(s.remote)
	s.connHandler.Close()
}

// MessageHandler returns a handler for processing a remote message, verifying and dispatching it,
// and writing the response to the connection. Handles protocol commands and errors.
func MessageHandler(pubSub PubSubConn, tenant, remote string) core.MessageHandler {

	return func(writer io.Writer, raw []byte, dataCh <-chan []byte, close chan<- struct{}) {

		data := bytes.TrimSpace(raw)

		var result []byte
		switch {
		// STOP: Handle connection close request from remote.
		case bytes.Equal(bytes.ToUpper(data), core.OpStop), bytes.Equal(data, core.ControlC):
			slog.Info("Closing connection", "remote", remote)
			// If remote sends STOP, close resources.
			close <- struct{}{}
			return

		// PING: Respond with PONG.
		case bytes.Equal(bytes.ToUpper(data), core.OpPing):
			result = core.BuildBytes(core.OpPong, core.CRLF)

		// PONG: Acknowledge with OK.
		case bytes.Equal(bytes.ToUpper(data), core.OpPong):
			result = core.OK

		// PUB: Handle publish command.
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpPub):
			result = handlePub(pubSub, data, dataCh, tenant)

		// SUB: Handle subscribe command.
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpSub):
			slog.Debug("sub", "value", data)
			result = handleSub(writer, pubSub, tenant, remote, data)

		// UNSUB: Handle unsubscribe command.
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpUnsub):
			result = handleUnsub(pubSub, remote, data)

		// NO DATA: Ignore empty messages.
		case bytes.Equal(data, core.Empty):
			return

		default:
			// UNKNOWN: Respond with protocol error.
			result = core.BuildBytes([]byte("-ERR invalid protocol"), core.CRLF)
		}

		_, err := writer.Write(result)
		if err != nil {
			if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
				return
			}
			slog.Error("server handler handleConnection", "error", err)
		}

	}
}

// handlePub handles PUB messages. See core.OpPub.
// It parses the subject and optional reply-to, reads the message payload, and dispatches it to the pubsub engine.
func handlePub(pubSub PubSubConn, received []byte, dataCh <-chan []byte, tenant string) []byte {
	// Default result
	result := core.OK

	// Get next line (message payload)
	msg := <-dataCh

	// Parse command arguments
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

	if tenant != "" {
		opts = append(opts, WithTenantPub(tenant))
	}

	// Dispatch message to pubsub engine
	pubSub.Publish(string(args[1]), msg, opts...)

	return result
}

// handleUnsub handles UNSUB commands. See core.OpUnsub.
// It parses the subject and subscription ID, and unsubscribes the handler from the pubsub engine.
func handleUnsub(pubSub PubSubConn, remote string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse command arguments
	args := bytes.Split(received, core.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// Dispatch unsubscribe to pubsub engine
	if err := pubSub.Unsubscribe(string(args[1]), remote, id); err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}
	return result
}

// handleSub handles SUB commands. See core.OpSub.
// It parses the subject, subscription ID, and optional group, and subscribes the handler to the pubsub engine.
func handleSub(writer io.Writer, pubSub PubSubConn, tenant string, remote string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse command arguments
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

	if tenant != "" {
		opts = append(opts, WithTenantSub(tenant))
	}

	// Dispatch subscribe to pubsub engine
	err := pubSub.Subscribe(string(args[1]), remote, subscriberHandler(writer, subID), opts...)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}

	return result
}

// subscriberHandler returns a Handler that writes received messages to the connection.
// It formats the message according to the protocol and logs errors if sending fails.
func subscriberHandler(writer io.Writer, sid int) Handler {
	return func(msg Message) {
		subID := strconv.AppendInt([]byte{}, int64(sid), 10)
		result := core.BuildBytes(core.OpMsg, core.Space, []byte(msg.Subject), core.Space, subID, core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF)
		slog.Debug("MSG message", "result", string(result))

		_, err := writer.Write(result)
		if err != nil {
			slog.Error("send", "error", err)
		}
	}
}
