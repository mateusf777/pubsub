package client

import "fmt"

type Handler func(*Message)

type pubSub struct {
	msgCh       chan *Message
	nextSub     int
	subscribers map[int]Handler
}

func newPubSub() *pubSub {
	return &pubSub{
		msgCh:       make(chan *Message),
		subscribers: make(map[int]Handler),
	}
}

func (ps *pubSub) publish(id int, msg *Message) error {
	if handle, ok := ps.subscribers[id]; ok {
		handle(msg)
		return nil
	}
	return fmt.Errorf("the subscriber %d was not found", id)
}

type Message struct {
	conn    *Conn
	Subject string
	Reply   string
	Data    []byte
}

func (m *Message) Respond(data []byte) error {
	return m.conn.Publish(m.Reply, data)
}
