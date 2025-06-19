package integration

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/mateusf777/pubsub/client"
	"github.com/stretchr/testify/assert"
)

func TestPublishTLS(t *testing.T) {
	done := make(chan struct{}, 1)

	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	subID, err := connSub.Subscribe("hello", func(msg *client.Message) {
		assert.Equal(t, "world", string(msg.Data))
		done <- struct{}{}
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub.Unsubscribe("hello", subID)

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	err = connPub.Publish("hello", []byte("world"))
	assert.Nil(t, err)

	err = connPub.Publish("hello", []byte("world"))
	assert.Nil(t, err)

	select {
	case <-done:
		t.Logf("connSub.Subscribe called")
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for handler call")
	}

}

func TestQueueTLS(t *testing.T) {
	const expected = 10
	done := make(chan struct{}, expected)
	done2 := make(chan struct{}, expected)

	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	sub1, err := connSub.QueueSubscribe("test", "queue", func(msg *client.Message) {
		assert.Equal(t, "message", string(msg.Data))
		done <- struct{}{}
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub.Unsubscribe("test", sub1)

	connSub2, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub2.Close()

	sub2, err := connSub2.QueueSubscribe("test", "queue", func(msg *client.Message) {
		assert.Equal(t, "message", string(msg.Data))
		done2 <- struct{}{}
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub2.Unsubscribe("test", sub2)

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	for range expected {
		err = connPub.Publish("test", []byte("message"))
		assert.Nil(t, err)
	}

	for i := 0; i < expected; i++ {
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

func TestRequestTLS(t *testing.T) {
	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	subID, err := connSub.Subscribe("test", func(msg *client.Message) {
		assert.Equal(t, "request", string(msg.Data))
		errReply := connSub.Reply(msg, []byte("response"))
		assert.Nil(t, errReply)
	})
	if err != nil {
		t.Error(err)
	}
	defer connSub.Unsubscribe("test", subID)

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	msg, err := connPub.Request("test", []byte("request"))
	assert.Nil(t, err)
	assert.Equal(t, "response", string(msg.Data))
}
