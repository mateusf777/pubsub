package server

import (
	"bytes"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

// PubSubConn provides an API to the PubSub engine.
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

type ConnectionHandler interface {
	Handle()
	Close()
}

type ConnHandler struct {
	connHandler ConnectionHandler
	pubSub      PubSubConn
	remote      string
}

func (s *ConnHandler) Run() {
	defer s.Close()
	s.connHandler.Handle()
}

func (s *ConnHandler) Close() {
	s.pubSub.UnsubAll(s.remote)
	s.connHandler.Close()
}

// MessageHandler return a handler for processing a remote message, verify and dispatch it and writes the response to the connection.
func MessageHandler(pubSub PubSubConn, remote string) core.MessageHandler {

	return func(writer io.Writer, raw []byte, dataCh <-chan []byte, close chan<- struct{}) {

		data := bytes.TrimSpace(raw)

		var result []byte
		switch {
		// STOP
		case bytes.Equal(bytes.ToUpper(data), core.OpStop), bytes.Equal(data, core.ControlC):
			slog.Info("Closing connection", "remote", remote)
			// If remote send STOP we close resources
			close <- struct{}{}
			return

		// PING
		case bytes.Equal(bytes.ToUpper(data), core.OpPing):
			result = core.BuildBytes(core.OpPong, core.CRLF)

		// PONG
		case bytes.Equal(bytes.ToUpper(data), core.OpPong):
			result = core.OK

		// PUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpPub):
			result = handlePub(pubSub, data, dataCh)

		// SUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpSub):
			slog.Debug("sub", "value", data)
			result = handleSub(writer, pubSub, remote, data)

		// UNSUB
		case bytes.HasPrefix(bytes.ToUpper(data), core.OpUnsub):
			result = handleUnsub(pubSub, remote, data)

		// NO DATA
		case bytes.Equal(data, core.Empty):
			return

		default:
			// UNKNOWN
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
func handleUnsub(pubSub PubSubConn, remote string, received []byte) []byte {
	// Default result
	result := core.OK

	// Parse
	args := bytes.Split(received, core.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// Dispatch
	if err := pubSub.Unsubscribe(string(args[1]), remote, id); err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}
	return result
}

// handleSub Handles SUB. See core.OpSub.
func handleSub(writer io.Writer, pubSub PubSubConn, remote string, received []byte) []byte {
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
	err := pubSub.Subscribe(string(args[1]), remote, subscriberHandler(writer, subID), opts...)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}

	return result
}

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
