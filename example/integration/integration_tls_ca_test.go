package integration

import (
	"crypto/tls"
	"os"
	"testing"

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
