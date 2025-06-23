package main

import (
	"crypto/rand"
	"crypto/tls"
	"log/slog"
	"os"
	"sync"

	"github.com/mateusf777/pubsub/client"

	"github.com/mateusf777/pubsub/example/common"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	wg := &sync.WaitGroup{}
	for i := 0; i < common.Routines; i++ {
		wg.Add(1)
		go send(wg)
	}

	wg.Wait()
}

func send(wg *sync.WaitGroup) {

	clientCert := os.Getenv("PUBSUB_TLS_CLIENT_CERT")
	clientKey := os.Getenv("PUBSUB_TLS_CLIENT_KEY")

	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		slog.Error("failed to load client cert/key", "error", err)
		return
	}

	conn, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{cert},
	}))
	if err != nil {
		slog.Error("connection", "error", err)
		return
	}
	defer wg.Done()
	defer func() {
		conn.Close()
		slog.Info("Connection closed")
	}()

	slog.Info("Sending messages", "count", common.Messages)
	for i := 0; i < common.Messages; i++ {
		msg := Generate512ByteString()
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			slog.Error("send Publish", "error", err)
			return
		}
		//time.Sleep(500 * time.Millisecond)
	}

	slog.Info("Done")
}

func Generate512ByteString() string {
	const size = 512
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:',.<>?/`~"

	b := make([]byte, size)
	charsetLen := byte(len(charset))

	randomBytes := make([]byte, size)
	if _, err := rand.Read(randomBytes); err != nil {
		return err.Error()
	}

	for i := range b {
		b[i] = charset[randomBytes[i]%charsetLen]
	}

	return string(b)
}
