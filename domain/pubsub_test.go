package domain

import (
	"testing"
	"time"
)

func TestNewPubSub(t *testing.T) {
	ps := NewPubSub()

	if ps.msgCh == nil {
		t.Errorf("channel not initiated")
	}

	if ps.handlersMap == nil {
		t.Errorf("handlers not initiated")
	}

	timer := time.NewTimer(time.Second)
	for {
		select {
		case <-timer.C:
			t.Errorf("the pubsub was not started and timed out")
		default:
			if ps.running {
				break
			}
		}
		break
	}
}

func TestPublishWithoutSub(t *testing.T) {
	ps := NewPubSub()

	err := ps.Publish("test", "")
	if err == nil {
		t.Errorf("should have returned an error")
	}
	t.Logf("%+v", err)
}

func TestSubscribe(t *testing.T) {
	ps := NewPubSub()

	err := ps.Subscribe("test", "client", func(msg Message) {
		t.Logf("%v", msg)
	})
	if err != nil {
		t.Errorf("should not return an error, %+v", err)
	}

	if _, ok := ps.handlersMap.Load("test"); !ok {
		t.Errorf("the subscriber was not correctly persisted")
	}

	if !ps.hasSubscriber("test") {
		t.Errorf("the subscriber was not correctly persisted")
	}
}

func TestReceiveMessage(t *testing.T) {
	ps := NewPubSub()
	received := false
	err := ps.Subscribe("test", "client", func(msg Message) {
		received = true
		if msg.Subject != "test" || msg.Value != "test" {
			t.Errorf("the message was not what the expected, %v", msg)
		}
		t.Logf("%v", msg)
	})
	if err != nil {
		t.Errorf("should not return an error, %+v", err)
	}

	err = ps.Publish("test", "test")
	if err != nil {
		t.Errorf("should not return an error, %+v", err)
	}

	timer := time.NewTimer(time.Second)
	for {
		select {
		case <-timer.C:
			t.Errorf("the message was not received and timed out")
		default:
			if received {
				break
			}
		}
		break
	}
}
