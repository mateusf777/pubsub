package client

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/domain"
)

func handleConnection(c *Conn, ctx context.Context, ps *pubSub) {
	defer func(c *Conn) {
		logger.Info("Closing connection", "remote", c.conn.RemoteAddr().String())
		c.drained <- struct{}{}
	}(c)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan []byte, 10)

	go func(ctx context.Context) {

		go domain.Read(c.conn, buffer, dataCh)

		for {
			select {
			case <-ctx.Done():
				stopTimeout <- true
				closeHandler <- true
				logger.Info("done!")
				return
			case netData := <-dataCh:
				logger.Debug("Received", "netData", netData)
				timeoutReset <- true

				temp := bytes.TrimSpace(netData)

				var result []byte
				switch {
				case domain.Equals(bytes.ToUpper(temp), domain.OpPing):
					result = bytes.Join([][]byte{domain.OpPong, domain.CRLF}, nil)
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpPong):
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpOK):
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpERR):
					logger.Error("OpERR", "value", temp)
					break

				case bytes.HasPrefix(bytes.ToUpper(temp), domain.OpMsg):
					logger.Debug("in opMsg...")
					logger.Debug("calling handleMsg")
					handleMsg(c, ps, temp, dataCh)

				default:
					if domain.Equals(temp, domain.Empty) {
						continue
					}
					result = domain.Empty
				}

				if !domain.Equals(result, domain.Empty) {
					_, err := c.conn.Write(result)
					if err != nil {
						if strings.Contains(err.Error(), "broken pipe") {
							continue
						}
						logger.Error("client handleConnection", "error", err)
					}
				}

			}
		}
	}(ctx)

	go domain.MonitorTimeout(c.conn, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler

	return
}

func handleMsg(c *Conn, ps *pubSub, received []byte, dataCh chan []byte) {
	logger.Debug("handleMsg", "received", received)
	parts := bytes.Split(received, domain.CRLF)
	args := bytes.Split(parts[0], domain.Space)
	msg := <-dataCh
	if len(args) < 3 || len(args) > 4 {
		return //"-ERR should be MSG <subject> <id> [reply-to]\n"
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

	err := ps.publish(id, message)
	if err != nil {
		return // fmt.Sprintf("-ERR %v\n", err)
	}
}
