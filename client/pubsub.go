package client

import (
	"fmt"
	"sync"
)

// Handler is a function to handle messages sent to subjects to which it's subscribed
type Handler func(*Message)

type msgRouter struct {
	mu sync.RWMutex
	subHandlers map[int]Handler
}

func newMsgRouter() *msgRouter {
	return &msgRouter{
		subHandlers: make(map[int]Handler),
	}
}

func (ps *msgRouter) route(msg *Message, subscriberID int) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if handle, ok := ps.subHandlers[subscriberID]; ok {
		handle(msg)
		return nil
	}
	return fmt.Errorf("the subscriber %d was not found", subscriberID)
}

func (ps *msgRouter) addSubHandler(handler Handler) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	nextSub := len(ps.subHandlers) + 1
	ps.subHandlers[nextSub] = handler
	return nextSub
}

func (ps *msgRouter) removeSubHandler(subscriberID int) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.subHandlers, subscriberID)
}
