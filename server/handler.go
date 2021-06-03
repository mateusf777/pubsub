package server

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/domain"
	"github.com/mateusf777/pubsub/log"
)

func handleConnection(c net.Conn, ps *domain.PubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			log.Error("%v\n", err)
		}
		log.Info("Closed connection %s\n", c.RemoteAddr().String())
	}(c)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	log.Info("Serving %s\n", c.RemoteAddr().String())
	client := c.RemoteAddr().String()

	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan string, 10)

	go func() {
	Loop:
		for {
			go psnet.Read(c, buffer, dataCh)

			// dispatch
			accumulator := psnet.Empty
			for netData := range dataCh {
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
				case strings.ToUpper(temp) == psnet.OpStop:
					log.Info("Closing connection with %s\n", c.RemoteAddr().String())
					stopTimeout <- true
					closeHandler <- true
					break Loop

				case strings.ToUpper(temp) == psnet.OpPing:
					result = psnet.OpPong + psnet.CRLF
					break

				case strings.ToUpper(temp) == psnet.OpPong:
					result = psnet.OK
					break

				case strings.HasPrefix(strings.ToUpper(temp), psnet.OpPub):
					// uses accumulator to get next line
					if accumulator == psnet.Empty {
						accumulator = temp
						continue
					}
					result = handlePub(c, ps, client, temp)

				case strings.HasPrefix(strings.ToUpper(temp), psnet.OpSub):
					log.Debug("sub: %v", temp)
					result = handleSub(c, ps, client, temp)

				case strings.HasPrefix(strings.ToUpper(temp), psnet.OpUnsub):
					result = handleUnsub(ps, client, temp)

				default:
					result = "-ERR invalid protocol" + psnet.CRLF
				}
				_, err := c.Write([]byte(result))
				if err != nil {
					log.Error("%v\n", err)
				}

				// reset accumulator
				accumulator = psnet.Empty
			}
		}
	}()

	go psnet.MonitorTimeout(c, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler
	ps.UnsubAll(client)
	return
}

func handlePub(c net.Conn, ps *domain.PubSub, client string, received string) string {
	// default result
	result := psnet.OK

	// parse
	parts := strings.Split(received, psnet.CRLF)
	args := strings.Split(parts[0], psnet.Space)
	msg := parts[1]

	if len(args) < 2 || len(args) > 3 {
		return "-ERR should be PUB <subject> [reply-to]\n"
	}

	opts := make([]domain.PubOpt, 0)
	// subscribe for reply
	if len(args) == 3 {
		reply := args[2]
		err := ps.Subscribe(reply, client, func(msg domain.Message) {

			result = fmt.Sprintf("MSG %s %s\r\n%v\r\n", msg.Subject, msg.Reply, msg.Value)
			log.Debug("pub sending %s", result)
			_, err := c.Write([]byte(result))
			if err != nil {
				log.Error("%v\n", err)
			}

		}, domain.WithMaxMsg(1))
		if err != nil {
			return fmt.Sprintf("-ERR %v\n", err)
		}
		opts = append(opts, domain.WithReply(reply))
	}

	// dispatch
	err := ps.Publish(args[1], msg, opts...)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}

	return result
}

func handleUnsub(ps *domain.PubSub, client string, received string) string {
	// default result
	result := psnet.OK

	// parse
	args := strings.Split(received, psnet.Space)
	if len(args) != 3 {
		return "-ERR should be UNSUB <subject> <id>\n"
	}
	id, _ := strconv.Atoi(args[2])

	// dispatch
	err := ps.Unsubscribe(args[1], client, id)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}
	return result
}

func handleSub(c net.Conn, ps *domain.PubSub, client string, received string) string {
	// default result
	result := psnet.OK

	// parse
	args := strings.Split(received, psnet.Space)
	if len(args) < 3 || len(args) > 5 {
		return "-ERR should be SUB <subject> <id> [max-msg] [group]\n"
	}

	id, _ := strconv.Atoi(args[2])
	opts := make([]domain.SubOpt, 0)

	if len(args) == 4 {
		maxMsg, _ := strconv.Atoi(args[3])
		opts = append(opts, domain.WithMaxMsg(maxMsg))
	}
	if len(args) == 5 {
		group := args[4]
		opts = append(opts, domain.WithGroup(group))
	}
	opts = append(opts, domain.WithID(id))

	// dispatch
	err := ps.Subscribe(args[1], client, func(msg domain.Message) {
		err := sendMsg(c, id, msg)
		if err != nil {
			log.Error("%v\n", err)
		}
	}, opts...)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}

	return result
}

func sendMsg(conn net.Conn, id int, msg domain.Message) error {
	result := fmt.Sprintf("MSG %s %d %s\r\n%v\r\n", msg.Subject, id, msg.Reply, msg.Value)
	log.Debug("sub sending %s", result)
	_, err := conn.Write([]byte(result))
	if err != nil {
		return err
	}
	return nil
}
