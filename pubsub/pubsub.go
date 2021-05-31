package pubsub

import (
	"fmt"
	"math/rand"

	"github.com/mateusf777/pubsub/log"
)

// PubSub represents a message router for handling the operations PUB, SUB and UNSUB
type PubSub struct {
	msgCh       chan Message
	handlersMap map[string][]HandlerSubject
	groupsMap   map[string][]HandlerSubject
	running     bool
}

// Message routed from a PUB op to handlers previously registerer by SUB ops
type Message struct {
	Subject string
	Reply   string
	Value   interface{}
}

// HandlerSubject represents a handler for a subject
type HandlerSubject struct {
	client   string
	id       int
	subject  string
	handler  Handler
	maxMsg   int
	countMsg int
	group    string
}

// Handler is the type of the function that will handle the message received from a PUB op
type Handler func(msg Message)

// NewPubSub creates a PubSub and starts the message route
func NewPubSub() *PubSub {
	ps := PubSub{
		msgCh:       make(chan Message),
		handlersMap: make(map[string][]HandlerSubject, 0),
		running:     false,
	}
	go ps.run()

	return &ps
}

// Stop the PubSub router
func (ps *PubSub) Stop() {
	close(ps.msgCh)
}

// PubOpt optional parameter pattern for Publish
type PubOpt func(*Message)

// WithReply returns a PubOpt that adds a optional reply received in a PUB op
func WithReply(subject string) PubOpt {
	return func(msg *Message) {
		msg.Reply = subject
	}
}

// Publish constructs a message based in a PUB op and sends it to be routed to the subscribers
// Returns an error if there are no subscribers for the subject
func (ps *PubSub) Publish(subject string, value interface{}, opts ...PubOpt) error {
	if !ps.hasSubscriber(subject) {
		return fmt.Errorf("no subscribers for this subject")
	}

	message := Message{
		Subject: subject,
		Value:   value,
	}
	for _, opt := range opts {
		opt(&message)
	}

	ps.msgCh <- message
	return nil
}

// SubOpt optional parameter pattern for Subscribe
type SubOpt func(*HandlerSubject)

// WithMaxMsg returns a SubOpt that populates HandlerSubject with the maxMsg
func WithMaxMsg(maxMsg int) SubOpt {
	return func(hs *HandlerSubject) {
		hs.maxMsg = maxMsg
	}
}

// WithGroup returns a SubOpt that populates HandlerSubject with the group
func WithGroup(group string) SubOpt {
	return func(hs *HandlerSubject) {
		hs.group = group
	}
}

// Subscribe register the subject subscribers received in a SUB op
func (ps *PubSub) Subscribe(subject string, client string, id int, handler Handler, opts ...SubOpt) error {
	if ps.handlersMap == nil {
		return fmt.Errorf("the pubsub was not correctly initiated, the list of handlers is nil")
	}
	if subject == "" || handler == nil {
		return fmt.Errorf("invalid parameters, both subject and handler need to be given")
	}

	if _, ok := ps.handlersMap[subject]; !ok {
		ps.handlersMap[subject] = make([]HandlerSubject, 0)
	}

	hs := HandlerSubject{
		client:  client,
		id:      id,
		subject: subject,
		handler: handler,
	}
	for _, opt := range opts {
		opt(&hs)
	}

	handlers := ps.handlersMap[subject]
	handlers = append(handlers, hs)
	ps.handlersMap[subject] = handlers
	return nil
}

// Unsubscribe removes the subject subscribers received in a UNSUB op
func (ps *PubSub) Unsubscribe(subject string, client string, id int) error {
	if ps.handlersMap == nil {
		return fmt.Errorf("the pubsub was not correctly initiated, the list of handlers is nil")
	}
	if subject == "" {
		return fmt.Errorf("invalid parameters, both subject and handler need to be given")
	}
	if _, ok := ps.handlersMap[subject]; !ok {
		return fmt.Errorf("there's no subscribers for subject[%s]", subject)
	}

	handlers := ps.handlersMap[subject]
	remove := -1
	for i, hs := range handlers {
		if hs.client == client && hs.id == id {
			remove = i
			break
		}
	}
	if remove == -1 {
		return fmt.Errorf("there's no subscriber for subject[%s], id[%d]", subject, id)
	}

	handlers[remove] = handlers[len(handlers)-1]
	ps.handlersMap[subject] = handlers[:len(handlers)-1]
	return nil
}

// run is the event loop that routes messages from PUB op to handlers registered by received SUB ops
// It's started by the NewPubSub
func (ps *PubSub) run() {

	log.Info("Message router started")
	ps.running = true

	defer func() {
		log.Info("Message router stopped")
		ps.running = false
	}()

	for msg := range ps.msgCh {
		ps.routeAndClean(msg)
	}
}

func (ps *PubSub) routeAndClean(msg Message) {
	subHandlers := ps.handlersMap[msg.Subject]

	// handlers with countMsg == maxMsg
	finishedHandlers := make([]int, 0)

	// handlers in a group to be rand load balanced
	groups := make(map[string][]HandlerSubject)

	for i, hs := range subHandlers {
		// prepare cleaning
		if hs.maxMsg > 0 {
			hs.countMsg += 1
			if hs.countMsg == hs.maxMsg {
				finishedHandlers = append(finishedHandlers, i)
			}
		}

		// prepare groups for load balancing
		if hs.group != "" {
			handlers := groups[hs.group]
			if handlers == nil {
				handlers = make([]HandlerSubject, 0)
			}
			handlers = append(handlers, hs)
			groups[hs.group] = handlers
			continue
		}

		// route
		hs.handler(msg)
	}

	// rand load balance by group
	for _, handlers := range groups {
		if handlers == nil {
			continue
		}
		hs := handlers[rand.Intn(len(handlers))]
		hs.handler(msg)
	}

	// execute cleaning
	for _, i := range finishedHandlers {
		subHandlers[i] = subHandlers[len(subHandlers)-1]
		ps.handlersMap[msg.Subject] = subHandlers[:len(subHandlers)-1]
	}
}

func (ps *PubSub) hasSubscriber(subject string) bool {
	if handlers, ok := ps.handlersMap[subject]; ok {
		return len(handlers) > 0
	}
	return false
}
