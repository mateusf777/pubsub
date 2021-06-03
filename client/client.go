package client

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/mateusf777/pubsub/log"
)

// Conn contains the state of the connection with the pubsub server to perform the necessary operations
type Conn struct {
	conn      net.Conn
	ps        *pubSub
	nextReply int
}

// Connect makes the connection with the server
func Connect(address string) (*Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	ps := newPubSub()

	nc := &Conn{
		conn: conn,
		ps:   ps,
	}
	go handleConnection(nc, ps)
	return nc, nil
}

// Close sends the "stop" operation so the server can clean up the resources
func (c *Conn) Close() {
	result := fmt.Sprintf("%v\r\n", "stop")
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		log.Error("%v", err)
	}
}

// Publish sends a message for a subject
func (c *Conn) Publish(subject string, msg []byte) error {
	result := fmt.Sprintf("PUB %s\r\n%v\r\n", subject, string(msg))
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return err
	}
	return nil
}

// Subscribe registers a handler that listen for messages sent to a subjets
func (c *Conn) Subscribe(subject string, handle Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	result := fmt.Sprintf("SUB %s %d\r\n", subject, c.ps.nextSub)
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	return err
}

// QueueSubscribe as subscribe, but the server will randomly load balance among the handlers in the queue
func (c *Conn) QueueSubscribe(subject string, queue string, handle Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	result := fmt.Sprintf("SUB %s %d %d %s\r\n", subject, c.ps.nextSub, -1, queue)
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	return err
}

// Request as publish, but blocks until receives a response from a subscriber
func (c *Conn) Request(subject string, msg []byte) (*Message, error) {
	resCh := make(chan *Message)
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = func(msg *Message) {
		log.Debug("received %v", msg)
		resCh <- msg
	}
	c.nextReply++
	reply := "REPLY." + strconv.Itoa(c.nextReply)

	result := fmt.Sprintf("SUB %s %d\r\n", reply, c.ps.nextSub)
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return nil, err
	}

	if msg == nil {
		msg = []byte("_")
	}
	log.Debug(reply)
	result = fmt.Sprintf("PUB %s %s\r\n%v\r\n", subject, reply, string(msg))
	log.Debug(result)
	_, err = c.conn.Write([]byte(result))
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(10 * time.Minute)
	select {
	case <-timeout.C:
		return nil, fmt.Errorf("timeout")
	case r := <-resCh:
		return r, nil
	}
}

func (c *Conn) PublishRequest(subject string, reply string, msg []byte) error {
	if msg == nil {
		msg = []byte("_")
	}
	result := fmt.Sprintf("PUB %s %s\r\n%v\r\n", subject, reply, string(msg))
	log.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return err
	}

	return nil
}
