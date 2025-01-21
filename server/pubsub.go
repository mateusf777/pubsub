package server

import (
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
)

const noIndex = -1

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

type Router interface {
	Route(subHandlers []HandlerSubject, msg Message)
}

// PubSub it's the engine that routes messages from publishers to subscribers.
type PubSub struct {
	msgCh       chan Message
	handlersMap *sync.Map
	router      Router
	running     bool
}

// PubSubConfig it's the optional configuration for creating a PubSub
type PubSubConfig struct {
	MsgCh       chan Message
	HandlersMap *sync.Map
}

// NewPubSub creates a PubSub and starts the message route
func NewPubSub(cfg PubSubConfig) *PubSub {
	if cfg.MsgCh == nil {
		cfg.MsgCh = make(chan Message)
	}

	if cfg.HandlersMap == nil {
		cfg.HandlersMap = new(sync.Map)
	}

	ps := PubSub{
		msgCh:       cfg.MsgCh,
		handlersMap: cfg.HandlersMap,
		router:      &msgRouter{},
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
func (ps *PubSub) Publish(subject string, data []byte, opts ...PubOpt) {
	if !ps.hasSubscriber(subject) {
		// If there's no subscriber it's ok: it doesn't return an error and short-circuit here.
		slog.Debug("There are no subscribers, message will be dropped")
		return
	}

	message := Message{
		Subject: subject,
		Data:    data,
	}
	for _, opt := range opts {
		opt(&message)
	}

	ps.msgCh <- message
}

// SubOpt optional parameter pattern for Subscribe
type SubOpt func(*HandlerSubject)

// WithGroup returns a SubOpt that populates HandlerSubject with the group
func WithGroup(group string) SubOpt {
	return func(hs *HandlerSubject) {
		hs.group = group
	}
}

// WithID returns a SubOpt that populates HandlerSubject with the id
func WithID(id int) SubOpt {
	return func(hs *HandlerSubject) {
		hs.id = id
	}
}

// Subscribe register the subject subscribers received in a SUB op
func (ps *PubSub) Subscribe(subject string, client string, handler Handler, opts ...SubOpt) error {
	if subject == "" || client == "" || handler == nil {
		return fmt.Errorf("invalid parameters, subject, client and handler need to be given")
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
	if subject == "" || client == "" {
		return fmt.Errorf("invalid parameters, subject, client and id need to be given")
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

// UnsubAll removes all subscriber handlers.
// Mainly used for cleanup before stopping a server.
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

	slog.Info("Message router started")
	ps.running = true

	defer func() {
		slog.Info("Message router stopped")
		ps.running = false
	}()

	for msg := range ps.msgCh {
		hs, _ := ps.handlersMap.Load(msg.Subject)
		subHandlers := hs.([]HandlerSubject)

		ps.router.Route(subHandlers, msg)
	}
}

type msgRouter struct{}

// Route find the subscriber handlers for the message subject, and pass the message to each one.
// If a subscriber handler is in a group, prepare the groups for load balance.
func (r *msgRouter) Route(subHandlers []HandlerSubject, msg Message) {

	// handlers in a group to be rand load balanced
	groups := make(map[string][]HandlerSubject)

	for _, hs := range subHandlers {
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

		// pass message to the handler
		hs.handler(msg)
	}

	// rand load balance by group
	for _, handlers := range groups {
		hs := handlers[rand.Intn(len(handlers))]
		hs.handler(msg)
	}
}

// hasSubscriber verifies if the handlersMap contains a handler for the subject
func (ps *PubSub) hasSubscriber(subject string) bool {
	if handlers, ok := ps.handlersMap.Load(subject); ok {
		return len(handlers.([]HandlerSubject)) > 0
	}
	return false
}
