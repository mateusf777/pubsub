package client

import (
	"net"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/log"
	psnet "github.com/mateusf777/pubsub/net"
)

func handleConnection(c *Conn, ps *pubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			log.Error("%v\n", err)
		}
		log.Info("Closed connection %s\n", c.RemoteAddr().String())
	}(c.conn)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan string, 10)

	go func() {
		for {
			go psnet.Read(c.conn, buffer, dataCh)

			accumulator := psnet.Empty
			for netData := range dataCh {
				log.Debug("Received %v", netData)
				timeoutReset <- true

				temp := strings.TrimSpace(netData)
				if temp == psnet.Empty {
					continue
				}

				if accumulator != psnet.Empty {
					temp = accumulator + psnet.CRLF + temp
				}

				var result string
				switch {
				case strings.ToUpper(temp) == psnet.OpPing:
					result = psnet.OpPong + "\n"
					break

				case strings.ToUpper(temp) == psnet.OpPong:
					break

				case strings.ToUpper(temp) == psnet.OpOK:
					break

				case strings.ToUpper(temp) == psnet.OpERR:
					log.Error(temp)
					break

				case strings.HasPrefix(strings.ToUpper(temp), psnet.OpMsg):
					log.Debug("in opMsg...")
					if accumulator == psnet.Empty {
						accumulator = temp
						continue
					}
					log.Debug("calling handleMsg")
					handleMsg(c, ps, temp)

				default:
					result = psnet.Empty
				}

				if result != psnet.Empty {
					_, err := c.conn.Write([]byte(result))
					if err != nil {
						log.Error("%v\n", err)
					}
				}

				accumulator = psnet.Empty

			}

		}
	}()

	go psnet.MonitorTimeout(c.conn, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler
	return
}

func handleMsg(c *Conn, ps *pubSub, received string) {
	log.Debug("handleMsg, %s", received)
	parts := strings.Split(received, psnet.CRLF)
	args := strings.Split(parts[0], psnet.Space)
	msg := parts[1]
	if len(args) < 3 || len(args) > 4 {
		return //"-ERR should be MSG <subject> <id> [reply-to]\n"
	}

	var reply string
	if len(args) == 4 {
		reply = args[3]
	}

	message := &Message{
		conn:    c,
		Subject: args[1],
		Reply:   reply,
		Data:    []byte(msg),
	}

	id, _ := strconv.Atoi(args[2])

	err := ps.publish(id, message)
	if err != nil {
		return // fmt.Sprintf("-ERR %v\n", err)
	}
}
