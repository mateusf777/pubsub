package client

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/domain"

	psnet "github.com/mateusf777/pubsub/net"
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

		go psnet.Read(c.conn, buffer, dataCh)
		accumulator := psnet.Empty

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

				if bytes.Compare(accumulator, psnet.Empty) != 0 {
					temp = domain.Join(accumulator, psnet.CRLF, temp)
				}

				var result []byte
				switch {
				case bytes.Compare(bytes.ToUpper(temp), psnet.OpPing) == 0:
					result = domain.Join(psnet.OpPong, psnet.CRLF)
					break

				case bytes.Compare(bytes.ToUpper(temp), psnet.OpPong) == 0:
					break

				case bytes.Compare(bytes.ToUpper(temp), psnet.OpOK) == 0:
					break

				case bytes.Compare(bytes.ToUpper(temp), psnet.OpERR) == 0:
					c.log.Error("%s", temp)
					break

				case bytes.HasPrefix(bytes.ToUpper(temp), psnet.OpMsg):
					c.log.Debug("in opMsg...")
					if bytes.Compare(bytes.ToUpper(accumulator), psnet.Empty) == 0 {
						accumulator = temp
						continue
					}
					c.log.Debug("calling handleMsg")
					handleMsg(c, ps, temp)

				default:
					if bytes.Compare(temp, psnet.Empty) == 0 {
						continue
					}
					result = psnet.Empty
				}

				if bytes.Compare(result, psnet.Empty) != 0 {
					_, err := c.conn.Write(result)
					if err != nil {
						if strings.Contains(err.Error(), "broken pipe") {
							continue
						}
						c.log.Error("client handleConnection, %v\n", err)
					}
				}

				accumulator = psnet.Empty
			}
		}
	}(ctx)

	go psnet.MonitorTimeout(c.conn, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler

	return
}

func handleMsg(c *Conn, ps *pubSub, received []byte) {
	c.log.Debug("handleMsg, %s", received)
	parts := bytes.Split(received, psnet.CRLF)
	args := bytes.Split(parts[0], psnet.Space)
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
