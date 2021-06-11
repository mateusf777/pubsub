package client

import (
	"fmt"

	psnet "github.com/mateusf777/pubsub/net"
)

// Handler is a function to handle messages sent to subjects to which it's subscribed
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

// Message contains data and metadata about a message sent from a publisher to a subscriber
type Message struct {
	conn    *Conn
	Subject string
	Reply   string
	Data    []byte
}

// Respond is a convenience method to respond to a requester
func (m *Message) Respond(data []byte) error {
	// Ignores responses when there's no reply subject
	if m.Reply == string(psnet.Empty) {
		return nil
	}
	return m.conn.Publish(m.Reply, data)
}
