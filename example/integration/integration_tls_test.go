package integration

import (
	"crypto/tls"
	"testing"

	"github.com/mateusf777/pubsub/client"
)

func TestPublishTLS(t *testing.T) {
	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	publish(t, connSub, connPub)
}

func TestQueueTLS(t *testing.T) {
	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	connSub2, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub2.Close()

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	queue(t, connSub, connSub2, connPub)
}

func TestRequestTLS(t *testing.T) {
	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{ServerName: "simpleappz.org"}))
	if err != nil {
		t.Error(err)
	}
	defer connPub.Close()

	request(t, connSub, connPub)
}
