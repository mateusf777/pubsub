package client

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

func MessageHandler(router router) core.MessageHandler {

	return func(writer io.Writer, raw []byte, dataCh <-chan []byte, _ chan<- struct{}) {

		l := logger.With("location", "Client MessageHandler")

		logger.Debug("Received", "netData", string(raw))

		data := bytes.TrimSpace(raw)

		var result []byte
		switch {
		case bytes.Equal(bytes.ToUpper(data), core.OpPing):
			result = bytes.Join([][]byte{core.OpPong, core.CRLF}, nil)

		case bytes.Equal(bytes.ToUpper(data), core.OpPong):
			return

		case bytes.Equal(bytes.ToUpper(data), core.OpOK):
			return

		case bytes.Equal(bytes.ToUpper(data), core.OpERR):
			l.Error("OpERR", "value", data)
			return

		case bytes.HasPrefix(bytes.ToUpper(data), core.OpMsg):
			l.Debug("in opMsg...")
			l.Debug("calling handleMsg")
			handleMsg(router, data, dataCh)

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
