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
		c.log.Info("Closing connection %s\n", c.conn.RemoteAddr().String())
		c.drained <- struct{}{}
	}(c)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan []byte, 10)

	go func(ctx context.Context) {

		go domain.Read(c.conn, buffer, dataCh)
		accumulator := domain.Empty

		for {
			select {
			case <-ctx.Done():
				stopTimeout <- true
				closeHandler <- true
				c.log.Info("done!")
				return
			case netData := <-dataCh:
				c.log.Debug("Received %v", netData)
				timeoutReset <- true

				temp := bytes.TrimSpace(netData)

				if !domain.Equals(accumulator, domain.Empty) {
					temp = domain.Join(accumulator, domain.CRLF, temp)
				}

				var result []byte
				switch {
				case domain.Equals(bytes.ToUpper(temp), domain.OpPing):
					result = domain.Join(domain.OpPong, domain.CRLF)
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpPong):
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpOK):
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpERR):
					c.log.Error("%s", temp)
					break

				case bytes.HasPrefix(bytes.ToUpper(temp), domain.OpMsg):
					c.log.Debug("in opMsg...")
					if domain.Equals(bytes.ToUpper(accumulator), domain.Empty) {
						accumulator = temp
						continue
					}
					c.log.Debug("calling handleMsg")
					handleMsg(c, ps, temp)

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
						c.log.Error("client handleConnection, %v\n", err)
					}
				}

				accumulator = domain.Empty
			}
		}
	}(ctx)

	go domain.MonitorTimeout(c.conn, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler

	return
}

func handleMsg(c *Conn, ps *pubSub, received []byte) {
	c.log.Debug("handleMsg, %s", received)
	parts := bytes.Split(received, domain.CRLF)
	args := bytes.Split(parts[0], domain.Space)
	msg := parts[1]
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
