package integration

import (
	"testing"
	"time"

	"github.com/mateusf777/pubsub/client"
	"github.com/stretchr/testify/assert"
)

func publish(t *testing.T, connSub *client.Client, connPub *client.Client) {
	done := make(chan struct{}, 1)

	subID, err := connSub.Subscribe("hello", func(msg *client.Message) {
		assert.Equal(t, "world", string(msg.Data))
		t.Logf("connSub.Subscribe called with message: %s", msg.Data)
		done <- struct{}{}
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub.Unsubscribe("hello", subID)

	time.Sleep(100 * time.Millisecond)
	err = connPub.Publish("hello", []byte("world"))
	assert.Nil(t, err)

	select {
	case <-done:
		t.Logf("connSub.Subscribe called")
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for handler call")
	}

}

func queue(t *testing.T, connSub *client.Client, connSub2 *client.Client, connPub *client.Client) {
	const expected = 10
	done := make(chan struct{}, expected)
	done2 := make(chan struct{}, expected)

	sub1, err := connSub.QueueSubscribe("test", "queue", func(msg *client.Message) {
		assert.Equal(t, "message", string(msg.Data))
		done <- struct{}{}
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub.Unsubscribe("test", sub1)

	sub2, err := connSub2.QueueSubscribe("test", "queue", func(msg *client.Message) {
		assert.Equal(t, "message", string(msg.Data))
		done2 <- struct{}{}
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub2.Unsubscribe("test", sub2)

	time.Sleep(100 * time.Millisecond)
	for range expected {
		err = connPub.Publish("test", []byte("message"))
		assert.Nil(t, err)
	}

	for i := range expected {
		select {
		case <-done:
			t.Logf("connSub.QueueSubscribe called")
		case <-done2:
			t.Logf("connSub2.QueueSubscribe called")
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting for handler call %d", i+1)
		}
	}
}

func request(t *testing.T, connSub *client.Client, connPub *client.Client) {
	subID, err := connSub.Subscribe("test", func(msg *client.Message) {
		assert.Equal(t, "request", string(msg.Data))
		errReply := connSub.Reply(msg, []byte("response"))
		assert.Nil(t, errReply)
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub.Unsubscribe("test", subID)

	time.Sleep(100 * time.Millisecond)
	msg, err := connPub.Request("test", []byte("request"))
	assert.Nil(t, err)
	assert.Equal(t, "response", string(msg.Data))
}
