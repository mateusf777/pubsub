package client

import (
	"fmt"

	"github.com/mateusf777/pubsub/core"
)

// Handler is a function to handle messages sent to subjects to which it's subscribed
type Handler func(*Message)

type msgRouter struct {
	msgCh       chan *Message
	subHandlers map[int]Handler
}

func newMsgRouter() *msgRouter {
	return &msgRouter{
		msgCh:       make(chan *Message),
		subHandlers: make(map[int]Handler),
	}
}

func (ps *msgRouter) route(msg *Message, subscriberID int) error {
	if handle, ok := ps.subHandlers[subscriberID]; ok {
		handle(msg)
		return nil
	}
	return fmt.Errorf("the subscriber %d was not found", subscriberID)
}

func (ps *msgRouter) addSubHandler(handler Handler) int {
	nextSub := len(ps.subHandlers) + 1
	ps.subHandlers[nextSub] = handler
	return nextSub
}

func (ps *msgRouter) removeSubHandler(subscriberID int) {
	delete(ps.subHandlers, subscriberID)
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
	if m.Reply == string(core.Empty) {
		return nil
	}
	return m.conn.Publish(m.Reply, data)
}
