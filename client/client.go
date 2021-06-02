package client

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
	opOK   = "+OK"
	opERR  = "-ERR"
	opPong = "PONG"
	opPing = "PING"
	opMsg  = "MSG"
)

type pubSub struct {
	msgCh       chan pubsub.Message
	nextSub     int
	subscribers map[int]pubsub.Handler
}

func newPubSub() *pubSub {
	return &pubSub{
		msgCh:       make(chan pubsub.Message),
		subscribers: make(map[int]pubsub.Handler),
	}
}

func (ps *pubSub) publish(id int, msg pubsub.Message) error {
	if handle, ok := ps.subscribers[id]; ok {
		handle(msg)
		return nil
	}
	return fmt.Errorf("the subscriber %d was not found", id)
}

type Conn struct {
	conn      net.Conn
	ps        *pubSub
	nextReply int
}

func Connect(address string) (*Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	ps := newPubSub()

	go handleConnection(conn, ps)

	return &Conn{
		conn: conn,
		ps:   ps,
	}, nil
}

func (c *Conn) Close() {
	result := fmt.Sprintf("%v\r\n", "stop")
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		log.Error("%v", err)
	}
}

func (c *Conn) Publish(subject string, msg []byte) error {
	result := fmt.Sprintf("PUB %s\r\n%v\r\n", subject, string(msg))
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) Subscribe(subject string, handle pubsub.Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	result := fmt.Sprintf("SUB %s %d\r\n", subject, c.ps.nextSub)
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	return err
}

func (c *Conn) Request(subject string, msg []byte) (pubsub.Message, error) {
	resCh := make(chan pubsub.Message)
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = func(msg pubsub.Message) {
		log.Info("received %v", msg)
		resCh <- msg
	}
	c.nextReply++
	reply := "REPLY." + strconv.Itoa(c.nextReply)

	result := fmt.Sprintf("SUB %s %d\r\n", reply, c.ps.nextSub)
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return pubsub.Message{}, err
	}

	if msg == nil {
		msg = []byte("_")
	}
	log.Info(reply)
	result = fmt.Sprintf("PUB %s %s\r\n%v\r\n", subject, reply, string(msg))
	log.Debug(result)
	_, err = c.conn.Write([]byte(result))
	if err != nil {
		return pubsub.Message{}, err
	}

	timeout := time.NewTimer(10 * time.Minute)
	select {
	case <-timeout.C:
		return pubsub.Message{}, fmt.Errorf("timeout")
	case r := <-resCh:
		return r, nil
	}
}

func handleConnection(c net.Conn, ps *pubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			log.Error("%v\n", err)
		}
		log.Info("Closed connection %s\n", c.RemoteAddr().String())
	}(c)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	timeout := time.NewTicker(ttl)
	timeoutCount := 0

	buffer := make([]byte, 1024)
	dataCh := make(chan string, 10)

	go func() {
		for {
			go func() {
				accumulator := ""
				for {
					n, err := c.Read(buffer)
					if err != nil {
						return
					}
					log.Debug("buffer :" + string(buffer[:n]))

					messages := strings.Split(accumulator+string(buffer[:n]), "\r\n")
					accumulator = ""

					log.Debug("messages :" + string(buffer[:n]))
					if !strings.HasSuffix(string(buffer[:n]), "\r\n") {
						accumulator = messages[len(messages)-1]
						messages = messages[:len(messages)-1]
					}

					log.Debug("messages %v\n", messages)
					for _, msg := range messages {
						log.Debug("pushing: %s", msg)
						dataCh <- msg
					}
				}
			}()

			accumulator := ""
			for netData := range dataCh {
				log.Debug("Received %v", netData)

				timeout.Reset(ttl)
				timeoutCount = 0

				temp := strings.TrimSpace(netData)
				if temp == "" {
					continue
				}

				if accumulator != "" {
					temp = accumulator + "\r\n" + temp
				}

				var result string
				switch {
				case strings.ToUpper(temp) == opPing:
					result = opPong + "\n"
					break

				case strings.ToUpper(temp) == opPong:
					break

				case strings.ToUpper(temp) == opOK:
					break

				case strings.ToUpper(temp) == opERR:
					log.Error(temp)
					break

				case strings.HasPrefix(strings.ToUpper(temp), opMsg):
					log.Debug("in opMsg...")
					if accumulator == "" {
						accumulator = temp
						continue
					}
					log.Debug("calling handleMsg")
					handleMsg(ps, temp)

				default:
					result = ""
				}

				if result != "" {
					_, err := c.Write([]byte(result))
					if err != nil {
						log.Error("%v\n", err)
					}
				}

				accumulator = ""

			}

		}
	}()

	go func() {
	Timeout:
		for {
			select {
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

				log.Debug("Sending timeout...")
				_, err := c.Write([]byte(opPing + "\r\n"))
				if err != nil {
					log.Error("%v\n", err)
				}

			}
		}
	}()

	<-closeHandler
	return
}

func handleMsg(ps *pubSub, received string) {
	log.Debug("handleMsg, %s", received)
	parts := strings.Split(received, "\r\n")
	args := strings.Split(parts[0], " ")
	msg := parts[1]
	if len(args) < 3 || len(args) > 4 {
		return //"-ERR should be MSG <subject> <id> [reply-to]\n"
	}

	var reply string
	if len(args) == 4 {
		reply = args[3]
	}

	message := pubsub.Message{
		Subject: args[1],
		Reply:   reply,
		Value:   msg,
	}

	id, _ := strconv.Atoi(args[2])

	err := ps.publish(id, message)
	if err != nil {
		return // fmt.Sprintf("-ERR %v\n", err)
	}

	//return result
}
