package client

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mateusf777/pubsub/log"
	psnet "github.com/mateusf777/pubsub/net"
	"github.com/mateusf777/pubsub/pubsub"
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

	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan string, 10)

	go func() {
		for {
			go psnet.Read(c, buffer, dataCh)

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
					handleMsg(ps, temp)

				default:
					result = psnet.Empty
				}

				if result != psnet.Empty {
					_, err := c.Write([]byte(result))
					if err != nil {
						log.Error("%v\n", err)
					}
				}

				accumulator = psnet.Empty

			}

		}
	}()

	go psnet.MonitorTimeout(c, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler
	return
}

func handleMsg(ps *pubSub, received string) {
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
}
