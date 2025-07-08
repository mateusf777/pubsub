package client

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

// MessageHandler returns a core.MessageHandler for the client.
// It processes protocol messages received from the server, handles PING/PONG/OK/ERR/MSG commands,
// and dispatches MSG messages to the router.
func MessageHandler(router router) core.MessageHandler {

	return func(writer io.Writer, raw []byte, dataCh <-chan []byte, _ chan<- struct{}) {

		l := logger.With("location", "Client MessageHandler")

		logger.Debug("Received", "netData", string(raw))

		data := bytes.TrimSpace(raw)

		var result []byte
		switch {
		case bytes.Equal(bytes.ToUpper(data), core.OpPing):
			// Respond to server PING with PONG.
			result = core.BuildBytes(core.OpPong, core.CRLF)

		case bytes.Equal(bytes.ToUpper(data), core.OpPong):
			// Ignore PONG responses.
			return

		case bytes.Equal(bytes.ToUpper(data), core.OpOK):
			// Ignore OK responses.
			return

		case bytes.Equal(bytes.ToUpper(data), core.OpERR):
			// Log protocol errors.
			l.Error("OpERR", "value", data)
			return

		case bytes.HasPrefix(bytes.ToUpper(data), core.OpMsg):
			// Handle MSG command: parse and route the message.
			l.Debug("in opMsg...")
			l.Debug("calling handleMsg")
			handleMsg(router, data, dataCh)
			return

		default:
			if bytes.Equal(data, core.Empty) {
				return
			}
			result = core.Empty
		}

		if !bytes.Equal(result, core.Empty) {
			_, err := writer.Write(result)
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					return
				}
				logger.Error("client handle", "error", err)
			}
		}
	}
}

// handleMsg parses a MSG protocol command and dispatches it to the appropriate subscription handler via the router.
// It extracts the subject, subscriber ID, optional reply-to, and payload from the message.
func handleMsg(router router, received []byte, dataCh <-chan []byte) {
	l := logger.With("location", "handleMsg()")

	l.Debug("routeMsg", "received", received)

	args := bytes.Split(received, core.Space)
	msg := <-dataCh

	if len(args) < 3 || len(args) > 4 {
		l.Debug("routeMsg", "args", args)
		return //"-ERR should be MSG <subject> <id> [reply-to] \n\r [payload] \n\r"
	}

	var reply []byte
	if len(args) == 4 {
		reply = args[3]
	}

	message := &Message{
		Subject: string(args[1]),
		Reply:   string(reply),
		Data:    msg,
	}

	id, _ := strconv.Atoi(string(args[2]))

	err := router.route(message, id)
	if err != nil {
		l.Error("route", "error", err)
		return // fmt.Sprintf("-ERR %v\n", err)
	}
}
