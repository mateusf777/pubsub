package server

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mateusf777/pubsub/log"

	"github.com/mateusf777/pubsub/pubsub"
)

const ttl = 5 * time.Minute

const (
	opStop  = "STOP"
	opPub   = "PUB"
	opSub   = "SUB"
	opUnsub = "UNSUB"
	opPong  = "PONG"
	opPing  = "PING"
)

const (
	crlf  = "\r\n"
	space = " "
	OK    = "+OK" + crlf
)

const closeErr = "use of closed network connection"

func handleConnection(c net.Conn, ps *pubsub.PubSub) {
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

	timeout := time.NewTicker(ttl)
	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan string, 10)

	go func() {
	Loop:
		for {
			go read(c, buffer, dataCh)

			// dispatch
			accumulator := ""
			for netData := range dataCh {
				timeoutReset <- true

				temp := strings.TrimSpace(netData)
				if temp == "" {
					continue
				}

				if accumulator != "" {
					temp = accumulator + crlf + temp
				}

				// operations
				var result string
				switch {
				case strings.ToUpper(temp) == opStop:
					log.Info("Closing connection with %s\n", c.RemoteAddr().String())
					stopTimeout <- true
					closeHandler <- true
					break Loop

				case strings.ToUpper(temp) == opPing:
					result = opPong + crlf
					break

				case strings.ToUpper(temp) == opPong:
					result = OK
					break

				case strings.HasPrefix(strings.ToUpper(temp), opPub):
					// uses accumulator to get next line
					if accumulator == "" {
						accumulator = temp
						continue
					}
					result = handlePub(c, ps, client, temp)

				case strings.HasPrefix(strings.ToUpper(temp), opSub):
					log.Debug("sub: %v", temp)
					result = handleSub(c, ps, client, temp)

				case strings.HasPrefix(strings.ToUpper(temp), opUnsub):
					result = handleUnsub(ps, client, temp)

				default:
					result = "-ERR invalid protocol" + crlf
				}
				_, err := c.Write([]byte(result))
				if err != nil {
					log.Error("%v\n", err)
				}

				// reset accumulator
				accumulator = ""

			}
		}
	}()

	go monitorTimeout(c, timeout, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler
	ps.UnsubAll(client)
	return
}

func monitorTimeout(c net.Conn, timeout *time.Ticker, timeoutReset chan bool, stopTimeout chan bool, closeHandler chan bool) {
	timeoutCount := 0
Timeout:
	for {
		select {
		case <-timeoutReset:
			timeout.Reset(ttl)
			timeoutCount = 0

		case <-stopTimeout:
			log.Debug("Stop timeout process %s\n", c.RemoteAddr().String())
			break Timeout

		case <-timeout.C:
			timeoutCount++
			if timeoutCount > 2 {
				log.Info("Timeout %s\n", c.RemoteAddr().String())
				closeHandler <- true
				break Timeout
			}

			_, err := c.Write([]byte(opPing + crlf))
			if err != nil {
				log.Error("%v\n", err)
			}

		}
	}
}

func read(c net.Conn, buffer []byte, dataCh chan string) {
	accumulator := ""
	for {
		n, err := c.Read(buffer)
		if err != nil {
			return
		}

		messages := strings.Split(accumulator+string(buffer[:n]), crlf)
		accumulator = ""

		if !strings.HasSuffix(string(buffer[:n]), crlf) {
			accumulator = messages[len(messages)-1]
			messages = messages[:len(messages)-1]
		}

		for _, msg := range messages {
			dataCh <- msg
		}
	}
}

func handlePub(c net.Conn, ps *pubsub.PubSub, client string, received string) string {
	// default result
	result := OK

	// parse
	parts := strings.Split(received, crlf)
	args := strings.Split(parts[0], space)
	msg := parts[1]

	if len(args) < 2 || len(args) > 3 {
		return "-ERR should be PUB <subject> [reply-to]\n"
	}

	opts := make([]pubsub.PubOpt, 0)
	// subscribe for reply
	if len(args) == 3 {
		reply := args[2]
		err := ps.Subscribe(reply, client, func(msg pubsub.Message) {

			result = fmt.Sprintf("MSG %s %s\r\n%v\r\n", msg.Subject, msg.Reply, msg.Value)
			log.Debug("pub sending %s", result)
			_, err := c.Write([]byte(result))
			if err != nil {
				log.Error("%v\n", err)
			}

		}, pubsub.WithMaxMsg(1))
		if err != nil {
			return fmt.Sprintf("-ERR %v\n", err)
		}
		opts = append(opts, pubsub.WithReply(reply))
	}

	// dispatch
	err := ps.Publish(args[1], msg, opts...)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}

	return result
}

func handleUnsub(ps *pubsub.PubSub, client string, received string) string {
	// default result
	result := OK

	// parse
	args := strings.Split(received, space)
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

func handleSub(c net.Conn, ps *pubsub.PubSub, client string, received string) string {
	// default result
	result := OK

	// parse
	args := strings.Split(received, space)
	if len(args) < 3 || len(args) > 5 {
		return "-ERR should be SUB <subject> <id> [max-msg] [group]\n"
	}

	id, _ := strconv.Atoi(args[2])
	opts := make([]pubsub.SubOpt, 0)

	if len(args) == 4 {
		maxMsg, _ := strconv.Atoi(args[3])
		opts = append(opts, pubsub.WithMaxMsg(maxMsg))
	}
	if len(args) == 5 {
		group := args[4]
		opts = append(opts, pubsub.WithGroup(group))
	}
	opts = append(opts, pubsub.WithID(id))

	// dispatch
	err := ps.Subscribe(args[1], client, func(msg pubsub.Message) {
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

func sendMsg(conn net.Conn, id int, msg pubsub.Message) error {
	result := fmt.Sprintf("MSG %s %d %s\r\n%v\r\n", msg.Subject, id, msg.Reply, msg.Value)
	log.Debug("sub sending %s", result)
	_, err := conn.Write([]byte(result))
	if err != nil {
		return err
	}
	return nil
}
