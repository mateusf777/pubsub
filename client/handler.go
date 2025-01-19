package client

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

func (c *Conn) handle(ctx context.Context) {
	defer func(c *Conn) {
		logger.Info("Closing connection", "remote", c.conn.RemoteAddr().String())
		c.drained <- struct{}{}
	}(c)

	closeHandler := make(chan bool)
	stopInactiveMonitor := make(chan bool)

	inactiveReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan []byte, 10)

	go func(ctx context.Context) {

		go core.Read(c.conn, buffer, dataCh)

		for {
			select {
			case <-ctx.Done():
				stopInactiveMonitor <- true
				closeHandler <- true
				logger.Info("done!")
				return
			case netData := <-dataCh:
				logger.Debug("Received", "netData", netData)
				inactiveReset <- true

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
					logger.Error("OpERR", "value", data)
					break

				case bytes.HasPrefix(bytes.ToUpper(data), core.OpMsg):
					logger.Debug("in opMsg...")
					logger.Debug("calling handleMsg")
					c.routeMsg(data, dataCh)

				default:
					if bytes.Equal(data, core.Empty) {
						continue
					}
					result = core.Empty
				}

				if !bytes.Equal(result, core.Empty) {
					_, err := c.conn.Write(result)
					if err != nil {
						if strings.Contains(err.Error(), "broken pipe") {
							continue
						}
						logger.Error("client handle", "error", err)
					}
				}

			}
		}
	}(ctx)

	go core.KeepAlive(c.conn, inactiveReset, stopInactiveMonitor, closeHandler)

	<-closeHandler

	return
}

func (c *Conn) routeMsg(received []byte, dataCh chan []byte) {
	logger.Debug("routeMsg", "received", received)

	args := bytes.Split(received, core.Space)
	msg := <-dataCh

	if len(args) < 3 || len(args) > 4 {
		logger.Debug("routeMsg", "args", args)
		return //"-ERR should be MSG <subject> <id> [reply-to] \n\r [payload] \n\r"
	}

	var reply []byte
	if len(args) == 4 {
		reply = args[3]
	}

	message := &Message{
		conn:    c,
		Subject: string(args[1]),
		Reply:   string(reply),
		Data:    msg,
	}

	id, _ := strconv.Atoi(string(args[2]))

	err := c.router.route(message, id)
	if err != nil {
		logger.Error("route", "error", err)
		return // fmt.Sprintf("-ERR %v\n", err)
	}
}
