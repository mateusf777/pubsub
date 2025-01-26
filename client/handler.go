package client

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

func (c *Conn) handle(ctx context.Context) {
	l := logger.With("location", "Conn.handle()")
	l.Debug("handle")

	defer func(c *Conn) {
		l.Info("Closing connection", "remote", c.conn.RemoteAddr().String())
		c.drained <- struct{}{}
	}(c)

	go c.connReader.Read()

	go c.msgProc.Process(ctx)

	go c.keepAlive.Run()

	<-c.closeHandler
	l.Info("After closeHandler")
}

type messageProcessor struct {
	writer          io.Writer
	router          router
	client          *Client
	data            chan []byte
	resetInactivity chan bool
	stopKeepAlive   chan bool
	closeHandler    chan bool
}

func (m *messageProcessor) Process(ctx context.Context) {
	l := logger.With("location", "messageProcessor.Process()")
	l.Debug("Process")

	for {
		select {
		case <-ctx.Done():
			m.stopKeepAlive <- true
			m.closeHandler <- true
			l.Info("done!")
			return

		case netData := <-m.data:
			l.Debug("Received", "netData", netData)
			m.resetInactivity <- true

			data := bytes.TrimSpace(netData)

			var result []byte
			switch {
			case bytes.Equal(bytes.ToUpper(data), core.OpPing):
				result = bytes.Join([][]byte{core.OpPong, core.CRLF}, nil)
				break

			case bytes.Equal(bytes.ToUpper(data), core.OpPong):
				break

			case bytes.Equal(bytes.ToUpper(data), core.OpOK):
				break

			case bytes.Equal(bytes.ToUpper(data), core.OpERR):
				l.Error("OpERR", "value", data)
				break

			case bytes.HasPrefix(bytes.ToUpper(data), core.OpMsg):
				l.Debug("in opMsg...")
				l.Debug("calling handleMsg")
				m.routeMsg(data, m.data)

			default:
				if bytes.Equal(data, core.Empty) {
					continue
				}
				result = core.Empty
			}

			if !bytes.Equal(result, core.Empty) {
				_, err := m.writer.Write(result)
				if err != nil {
					if strings.Contains(err.Error(), "broken pipe") {
						continue
					}
					logger.Error("client handle", "error", err)
				}
			}

		}
	}
}

func (m *messageProcessor) routeMsg(received []byte, dataCh chan []byte) {
	l := logger.With("location", "messageProcessor.routeMsg()")

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
		client:  m.client,
		Subject: string(args[1]),
		Reply:   string(reply),
		Data:    msg,
	}

	id, _ := strconv.Atoi(string(args[2]))

	err := m.router.route(message, id)
	if err != nil {
		l.Error("route", "error", err)
		return // fmt.Sprintf("-ERR %v\n", err)
	}
}
