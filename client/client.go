package client

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mateusf777/pubsub/core"
)

// Conn contains the state of the connection with the pubsub server to perform the necessary operations
type Conn struct {
	conn      net.Conn
	ps        *pubSub
	cancel    context.CancelFunc
	drained   chan struct{}
	nextReply int
}

var logger *slog.Logger

func SetLogLevel(level slog.Level) {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger = slog.New(logHandler)
}

func init() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	logger = slog.New(logHandler)
}

// Connect makes the connection with the server
func Connect(address string) (*Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	ps := newPubSub()

	// TODO: this probably need configuration. Unit tests will benefit from that
	ctx, cancel := context.WithCancel(context.Background())
	nc := &Conn{
		conn:    conn,
		ps:      ps,
		cancel:  cancel,
		drained: make(chan struct{}),
	}

	// TODO: this handleConnection order of parameters is strange. ctx should be the first one.
	// Why it is not the first one?
	go handleConnection(nc, ctx, ps)
	return nc, nil
}

// Close sends the "stop" operation so the server can clean up the resources
func (c *Conn) Close() {
	_, err := c.conn.Write(core.Stop)
	if err != nil {
		if !strings.Contains(err.Error(), "broken pipe") {
			logger.Error("Conn.Close", "error", err)
		}
	} else {
		for {
			_, err := c.conn.Write(core.Ping)
			if err != nil {
				continue
			}
			break
		}
	}

	slog.Info("close")
	c.cancel()
	<-c.drained
}

// Drain message and connections
func (c *Conn) Drain() {
	_, _ = c.conn.Write(core.Stop)
	for {
		_, err := c.conn.Write(core.Ping)
		if err == nil {
			continue
		}
		break
	}

	logger.Info("drained")
	c.cancel()
	<-c.drained
}

// Publish sends a message for a subject
// Uses command PUB <subject> \n\r message \n\r
func (c *Conn) Publish(subject string, msg []byte) error {
	result := bytes.Join([][]byte{core.OpPub, core.Space, []byte(subject), core.CRLF, msg, core.CRLF}, nil)
	slog.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return fmt.Errorf("client Publish, %v", err)
	}

	return nil
}

// Subscribe registers a handler that listen for messages sent to a subjets
// Uses command SUB <subject> <sub.ID>
func (c *Conn) Subscribe(subject string, handle Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	// TODO: why publish is using core.OpPub but SUB is not?
	result := fmt.Sprintf("SUB %s %d\r\n", subject, c.ps.nextSub)
	slog.Debug(result)

	if _, err := c.conn.Write([]byte(result)); err != nil {
		return fmt.Errorf("client Subscribe, %v", err)
	}

	return nil
}

// QueueSubscribe as subscribe, but the server will randomly load balance among the handlers in the queue
// Uses SUB <subject> <sub.ID> <queue>
func (c *Conn) QueueSubscribe(subject string, queue string, handle Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	// TODO: why publish is using core.OpPub but SUB is not?
	result := fmt.Sprintf("SUB %s %d %d %s\r\n", subject, c.ps.nextSub, -1, queue)
	slog.Debug(result)

	if _, err := c.conn.Write([]byte(result)); err != nil {
		return fmt.Errorf("client QueueSubscribe, %v", err)
	}
	return nil
}

// Request as publish, but blocks until receives a response from a subscriber
// Creates a subscription to receive reply: SUB <subject> REPLY.<ID>
// Then publishes request PUB <subject> REPLY.<ID> \n\r message \n\r
func (c *Conn) Request(subject string, msg []byte) (*Message, error) {
	resCh := make(chan *Message)
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = func(msg *Message) {
		slog.Debug("received", "msg", msg)
		resCh <- msg
	}

	c.nextReply++
	reply := "REPLY." + strconv.Itoa(c.nextReply)
	slog.Debug(reply)

	// TODO: why publish is using core.OpPub but SUB is not?
	result := fmt.Sprintf("SUB %s %d\r\n", reply, c.ps.nextSub)
	slog.Debug(result)

	if _, err := c.conn.Write([]byte(result)); err != nil {
		return nil, fmt.Errorf("client Request SUB, %v", err)
	}

	bResult := bytes.Join([][]byte{core.OpPub, core.Space, []byte(subject), core.Space, []byte(reply), core.CRLF, msg, core.CRLF}, nil)
	slog.Debug(string(bResult))

	if _, err := c.conn.Write(bResult); err != nil {
		return nil, fmt.Errorf("client Request PUB, %v", err)
	}

	// TODO: move this to context since it's a client lib
	timeout := time.NewTimer(10 * time.Minute)
	select {
	case <-timeout.C:
		return nil, fmt.Errorf("timeout")
	case r := <-resCh:
		return r, nil
	}
}
