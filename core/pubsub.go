package core

import (
	"fmt"
	"math/rand"
	"sync"
)

const noIndex = -1

// PubSub represents a message router for handling the operations PUB, SUB and UNSUB
type PubSub struct {
	msgCh       chan Message
	handlersMap *sync.Map //map[string][]HandlerSubject
	groupsMap   *sync.Map //map[string][]HandlerSubject
	running     bool
}

// Message routed from a PUB op to handlers previously registerer by SUB ops
type Message struct {
	Subject string
	Reply   string
	Data    []byte
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
		handlersMap: new(sync.Map),
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
func (ps *PubSub) Publish(subject string, data []byte, opts ...PubOpt) error {
	if !ps.hasSubscriber(subject) {
		// if there's no subscriber it's ok and it doesn't return an error, but short-circuit here
		return nil
	}

	message := Message{
		Subject: subject,
		Data:    data,
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

func WithID(id int) SubOpt {
	return func(hs *HandlerSubject) {
		hs.id = id
	}
}

// Subscribe register the subject subscribers received in a SUB op
func (ps *PubSub) Subscribe(subject string, client string, handler Handler, opts ...SubOpt) error {
	if ps.handlersMap == nil {
		return fmt.Errorf("the pubsub was not correctly initiated, the list of handlers is nil")
	}
	if subject == "" || handler == nil {
		return fmt.Errorf("invalid parameters, both subject and handler need to be given")
	}

	if _, ok := ps.handlersMap.Load(subject); !ok {
		ps.handlersMap.Store(subject, make([]HandlerSubject, 0))
	}

	hs := HandlerSubject{
		client:  client,
		subject: subject,
		handler: handler,
	}
	for _, opt := range opts {
		opt(&hs)
	}

	handlers, _ := ps.handlersMap.Load(subject)
	handlers = append(handlers.([]HandlerSubject), hs)
	ps.handlersMap.Store(subject, handlers)
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
	if _, ok := ps.handlersMap.Load(subject); !ok {
		return fmt.Errorf("there's no subscribers for subject[%s]", subject)
	}

	handlers, _ := ps.handlersMap.Load(subject)
	remove := noIndex
	for i, hs := range handlers.([]HandlerSubject) {
		if hs.client == client && hs.id == id {
			remove = i
			break
		}
	}
	if remove == noIndex {
		return fmt.Errorf("there's no subscriber for subject[%s], id[%d]", subject, id)
	}

	hs := handlers.([]HandlerSubject)
	hs[remove] = hs[len(hs)-1]
	ps.handlersMap.Store(subject, hs[:len(hs)-1])
	return nil
}

func (ps *PubSub) UnsubAll(client string) {
	ps.handlersMap.Range(func(subject, hs interface{}) bool {
		handlers := hs.([]HandlerSubject)
		size := len(handlers)
		count := 0
		for i := 0; i < size; i++ {
			if handlers[i].client == client {
				handlers[i] = handlers[size-1]
				count++
			}
		}
		cleanHs := handlers[:size-count]
		ps.handlersMap.Store(subject, cleanHs)
		return true
	})
}

// run is the event loop that routes messages from PUB op to handlers registered by received SUB ops
// It's started by the NewPubSub
func (ps *PubSub) run() {

	logger.Info("Message router started")
	ps.running = true

	defer func() {
		logger.Info("Message router stopped")
		ps.running = false
	}()

	for msg := range ps.msgCh {
		ps.route(msg)
	}
}

// TODO: some documentation to explain what it does wouldn't be bad
func (ps *PubSub) route(msg Message) {
	hs, _ := ps.handlersMap.Load(msg.Subject)

	// handlers with countMsg == maxMsg
	finishedHandlers := make([]int, 0)

	// handlers in a group to be rand load balanced
	groups := make(map[string][]HandlerSubject)

	subHandlers := hs.([]HandlerSubject)
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
		ps.handlersMap.Store(msg.Subject, subHandlers[:len(subHandlers)-1])
	}
}

func (ps *PubSub) hasSubscriber(subject string) bool {
	if handlers, ok := ps.handlersMap.Load(subject); ok {
		return len(handlers.([]HandlerSubject)) > 0
	}
	return false
}
