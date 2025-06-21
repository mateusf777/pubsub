package integration

import (
	"crypto/tls"
	"os"
	"testing"
	"time"

	"github.com/mateusf777/pubsub/client"
	"github.com/stretchr/testify/assert"
)

func TestConnectTLSNoCert(t *testing.T) {

	conn, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName: "simpleappz.org",
	}))
	t.Logf("conn: %v, err: %v", conn, err)
	assert.Nil(t, conn)
	assert.NotNil(t, err)
}

func TestConnectTLSInvalidCert(t *testing.T) {
	invalidClientCert := os.Getenv("PUBSUB_TLS_INVALID_CLIENT_CERT")
	invalidClientKey := os.Getenv("PUBSUB_TLS_INVALID_CLIENT_KEY")

	cert, err := tls.LoadX509KeyPair(invalidClientCert, invalidClientKey)
	if err != nil {
		t.Fatalf("failed to load client cert/key (%s,%s): %v", invalidClientCert, invalidClientKey, err)
	}

	conn, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{cert},
	}))
	t.Logf("conn: %v, err: %v", conn, err)
	assert.Nil(t, conn)
	assert.NotNil(t, err)
}

func TestConnectTLS(t *testing.T) {
	clientCert := os.Getenv("PUBSUB_TLS_CLIENT_CERT")
	clientKey := os.Getenv("PUBSUB_TLS_CLIENT_KEY")

	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("failed to load client cert/key: %v", err)
	}

	conn, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{cert},
	}))
	if err != nil {
		t.Error(err)
		return
	}
	assert.NotNil(t, conn)
	conn.Close()
}

func TestPublishDifferentTenants(t *testing.T) {
	clientCert := os.Getenv("PUBSUB_TLS_CLIENT_CERT")
	clientKey := os.Getenv("PUBSUB_TLS_CLIENT_KEY")

	clientCertb := os.Getenv("PUBSUB_TLS_CLIENT_CERT_B")
	clientKeyb := os.Getenv("PUBSUB_TLS_CLIENT_KEY_B")

	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("failed to load client cert/key: %v", err)
	}

	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{cert},
	}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	certb, err := tls.LoadX509KeyPair(clientCertb, clientKeyb)
	if err != nil {
		t.Fatalf("failed to load client cert/key: %v", err)
	}

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{certb},
	}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connPub.Close()

	publishMiss(t, connSub, connPub)
}

func publishMiss(t *testing.T, connSub *client.Client, connPub *client.Client) {
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
		t.Fatalf("connSub.Subscribe called")
	case <-time.After(time.Second):
		t.Log("timeout waiting for handler call")
	}

}

func TestPublishSameTenants(t *testing.T) {
	clientCert := os.Getenv("PUBSUB_TLS_CLIENT_CERT")
	clientKey := os.Getenv("PUBSUB_TLS_CLIENT_KEY")

	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("failed to load client cert/key: %v", err)
	}

	connSub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{cert},
	}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connSub.Close()

	certb, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("failed to load client cert/key: %v", err)
	}

	connPub, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{certb},
	}))
	if err != nil {
		t.Error(err)
		return
	}
	defer connPub.Close()

	publish(t, connSub, connPub)
}
