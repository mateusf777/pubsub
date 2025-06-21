package integration

import (
	"testing"

	"github.com/mateusf777/pubsub/client"
)

func TestPublish(t *testing.T) {
	connSub, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	connPub, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	publish(t, connSub, connPub)
}

func TestQueue(t *testing.T) {
	connSub, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	connSub2, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub2.Close()

	connPub, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	queue(t, connSub, connSub2, connPub)
}

func TestRequest(t *testing.T) {
	connSub, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	connPub, err := client.Connect(":9999")
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	request(t, connSub, connPub)
}
