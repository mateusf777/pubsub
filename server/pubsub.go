package server

import (
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
)

const noIndex = -1 // Sentinel value for not found index in handler slices.

// Message routed from a PUB op to handlers previously registerer by SUB ops.
// Represents a message in the pubsub system, including subject, reply subject, tenant, and payload.
type Message struct {
	Subject string // The subject/topic of the message.
	Reply   string // Optional reply subject for request/reply patterns.
	Tenant  string // Tenant identifier for multi-tenancy support.
	Data    []byte // Message payload.
}

// HandlerSubject represents a handler for a subject.
// Associates a client, subscription id, subject, handler function, group, and tenant.
type HandlerSubject struct {
	client  string  // Client identifier.
	id      int     // Subscription ID.
	subject string  // Subject/topic this handler is subscribed to.
	handler Handler // Function to handle incoming messages.
	group   string  // Optional group for queue subscriptions.
	tenant  string  // Tenant identifier for multi-tenancy.
}

// Handler is the type of the function that will handle the message received from a PUB op.
// Used to process messages delivered to subscribers.
type Handler func(msg Message)

// Router defines the interface for routing messages to handlers.
// Allows for custom routing strategies (e.g., load balancing, filtering).
type Router interface {
	Route(subHandlers []HandlerSubject, msg Message)
}

// PubSub it's the engine that routes messages from publishers to subscribers.
// Manages message delivery, handler registration, and routing logic.
type PubSub struct {
	msgCh       chan Message // Channel for incoming messages to be routed.
	handlersMap *sync.Map    // Map of subject -> []HandlerSubject.
	router      Router       // Router implementation for message delivery.
}

// PubSubConfig it's the optional configuration for creating a PubSub.
// Allows injection of custom message channel and handler map.
type PubSubConfig struct {
	MsgCh       chan Message // Optional custom message channel.
	HandlersMap *sync.Map    // Optional custom handler map.
}

// NewPubSub creates a PubSub and starts the message route.
// Initializes the engine with optional configuration and launches the router goroutine.
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
	}

	go ps.run()

	return &ps
}

// Stop the PubSub router.
// Closes the message channel, stopping the routing goroutine.
func (ps *PubSub) Stop() {
	close(ps.msgCh)
}

// PubOpt optional parameter pattern for Publish.
// Allows for flexible message construction (e.g., setting reply or tenant).
type PubOpt func(*Message)

// WithReply returns a PubOpt that adds a optional reply received in a PUB op.
// Used to set the reply subject for request/reply patterns.
func WithReply(subject string) PubOpt {
	return func(msg *Message) {
		msg.Reply = subject
	}
}

// WithTenantPub returns a PubOpt that sets the tenant for the published message.
func WithTenantPub(tenant string) PubOpt {
	return func(msg *Message) {
		msg.Tenant = tenant
	}
}

// Publish constructs a message based in a PUB op and sends it to be routed to the subscribers.
// If there are no subscribers for the subject, the message is dropped.
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

// SubOpt optional parameter pattern for Subscribe.
// Allows for flexible handler registration (e.g., setting group, id, tenant).
type SubOpt func(*HandlerSubject)

// WithGroup returns a SubOpt that populates HandlerSubject with the group.
// Used for queue group subscriptions (load balancing).
func WithGroup(group string) SubOpt {
	return func(hs *HandlerSubject) {
		hs.group = group
	}
}

// WithID returns a SubOpt that populates HandlerSubject with the id.
// Used to set the subscription ID for the handler.
func WithID(id int) SubOpt {
	return func(hs *HandlerSubject) {
		hs.id = id
	}
}

// WithTenantSub returns a SubOpt that sets the tenant for the handler subject.
func WithTenantSub(tenant string) SubOpt {
	return func(hs *HandlerSubject) {
		hs.tenant = tenant
	}
}

// Subscribe register the subject subscribers received in a SUB op.
// Adds a handler for the given subject and client, with optional options for group, id, and tenant.
func (ps *PubSub) Subscribe(subject string, client string, handler Handler, opts ...SubOpt) error {
	if subject == "" || client == "" || handler == nil {
		return fmt.Errorf("invalid parameters, subject, remote and handler need to be given")
	}

	slog.Debug("Subscribing", "subject", subject, "remote", client)

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

// Unsubscribe removes the subject subscribers received in a UNSUB op.
// Removes a handler for the given subject, client, and subscription id.
func (ps *PubSub) Unsubscribe(subject string, client string, id int) error {
	if subject == "" || client == "" {
		return fmt.Errorf("invalid parameters, subject, remote and id need to be given")
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
// Removes all handlers associated with a given client.
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
// Continuously reads messages from the channel and routes them to the appropriate handlers.
func (ps *PubSub) run() {

	slog.Info("Message router started")

	defer func() {
		slog.Info("Message router stopped")
	}()

	for msg := range ps.msgCh {
		hs, _ := ps.handlersMap.Load(msg.Subject)
		subHandlers := hs.([]HandlerSubject)

		ps.router.Route(subHandlers, msg)
	}
}

// msgRouter is the default Router implementation for PubSub.
// Handles routing logic, including group-based load balancing.
type msgRouter struct{}

// Route find the subscriber handlers for the message subject, and pass the message to each one.
// If a subscriber handler is in a group, prepare the groups for load balance.
// Delivers the message to all matching handlers, and load balances among group members.
func (r *msgRouter) Route(subHandlers []HandlerSubject, msg Message) {

	// handlers in a group to be rand load balanced
	groups := make(map[string][]HandlerSubject)

	for _, hs := range subHandlers {
		if hs.tenant != "" && hs.tenant != msg.Tenant {
			slog.Debug("Skipping handler for different tenant", "handler_tenant", hs.tenant, "message_tenant", msg.Tenant)
			continue
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
// Returns true if there is at least one handler for the given subject.
func (ps *PubSub) hasSubscriber(subject string) bool {
	if handlers, ok := ps.handlersMap.Load(subject); ok {
		return len(handlers.([]HandlerSubject)) > 0
	}
	return false
}
