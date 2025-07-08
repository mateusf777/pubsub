package client

import (
	"fmt"
	"sync"
)

// Handler is a function to handle messages sent to subjects to which it's subscribed.
// Used as a callback for delivering messages to subscribers.
type Handler func(*Message)

// msgRouter manages subscription handlers and routes messages to them.
// It is the default implementation of the router interface for the client.
type msgRouter struct {
	mu          sync.RWMutex    // Protects access to subHandlers.
	subHandlers map[int]Handler // Map of subscriber ID to Handler.
}

// newMsgRouter creates and initializes a new msgRouter instance.
func newMsgRouter() *msgRouter {
	return &msgRouter{
		subHandlers: make(map[int]Handler),
	}
}

// route delivers a message to the handler registered for the given subscriberID.
// Returns an error if the subscriber is not found.
func (ps *msgRouter) route(msg *Message, subscriberID int) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if handle, ok := ps.subHandlers[subscriberID]; ok {
		handle(msg)
		return nil
	}
	return fmt.Errorf("the subscriber %d was not found", subscriberID)
}

// addSubHandler registers a new handler and returns its subscriber ID.
// The subscriber ID is a unique integer for this router instance.
func (ps *msgRouter) addSubHandler(handler Handler) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	nextSub := len(ps.subHandlers) + 1
	ps.subHandlers[nextSub] = handler
	return nextSub
}

// removeSubHandler removes the handler associated with the given subscriber ID.
func (ps *msgRouter) removeSubHandler(subscriberID int) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.subHandlers, subscriberID)
}
