package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mateusf777/pubsub/domain"
)

// Conn contains the state of the connection with the pubsub server to perform the necessary operations
type Conn struct {
	conn      net.Conn
	ps        *pubSub
	cancel    context.CancelFunc
	drained   chan struct{}
	nextReply int
}

type Opt func(c *Conn)

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
func Connect(address string, opts ...Opt) (*Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	ps := newPubSub()

	ctx, cancel := context.WithCancel(context.Background())
	nc := &Conn{
		conn:    conn,
		ps:      ps,
		cancel:  cancel,
		drained: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(nc)
	}

	go handleConnection(nc, ctx, ps)
	return nc, nil
}

// Close sends the "stop" operation so the server can clean up the resources
func (c *Conn) Close() {
	_, err := c.conn.Write(domain.Stop)
	if err != nil {
		if !strings.Contains(err.Error(), "broken pipe") {
			logger.Error("Conn.Close", "error", err)
		}
	} else {
		for {
			_, err := c.conn.Write(domain.Ping)
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

// Drain ...
func (c *Conn) Drain() {
	_, _ = c.conn.Write(domain.Stop)
	for {
		_, err := c.conn.Write(domain.Ping)
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
func (c *Conn) Publish(subject string, msg []byte) error {
	result := domain.Join(domain.OpPub, domain.Space, []byte(subject), domain.CRLF, msg, domain.CRLF)
	slog.Debug(string(result))
	_, err := c.conn.Write(result)
	if err != nil {
		return fmt.Errorf("client Publish, %v", err)
	}
	return nil
}

// Subscribe registers a handler that listen for messages sent to a subjets
func (c *Conn) Subscribe(subject string, handle Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	result := fmt.Sprintf("SUB %s %d\r\n", subject, c.ps.nextSub)
	slog.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return fmt.Errorf("client Subscribe, %v", err)
	}
	return nil
}

// QueueSubscribe as subscribe, but the server will randomly load balance among the handlers in the queue
func (c *Conn) QueueSubscribe(subject string, queue string, handle Handler) error {
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = handle
	result := fmt.Sprintf("SUB %s %d %d %s\r\n", subject, c.ps.nextSub, -1, queue)
	slog.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return fmt.Errorf("client QueueSubscribe, %v", err)
	}
	return nil
}

// Request as publish, but blocks until receives a response from a subscriber
func (c *Conn) Request(subject string, msg []byte) (*Message, error) {
	resCh := make(chan *Message)
	c.ps.nextSub++
	c.ps.subscribers[c.ps.nextSub] = func(msg *Message) {
		slog.Debug("received", "msg", msg)
		resCh <- msg
	}
	c.nextReply++
	reply := "REPLY." + strconv.Itoa(c.nextReply)

	result := fmt.Sprintf("SUB %s %d\r\n", reply, c.ps.nextSub)
	slog.Debug(result)
	_, err := c.conn.Write([]byte(result))
	if err != nil {
		return nil, fmt.Errorf("client Request SUB, %v", err)
	}

	slog.Debug(reply)
	bResult := domain.Join(domain.OpPub, domain.Space, []byte(subject), domain.Space, []byte(reply), domain.CRLF, msg, domain.CRLF)
	slog.Debug(string(bResult))
	_, err = c.conn.Write(bResult)
	if err != nil {
		return nil, fmt.Errorf("client Request PUB, %v", err)
	}

	timeout := time.NewTimer(10 * time.Minute)
	select {
	case <-timeout.C:
		return nil, fmt.Errorf("timeout")
	case r := <-resCh:
		return r, nil
	}
}

func (c *Conn) PublishRequest(subject string, reply string, msg []byte) error {
	result := domain.Join(domain.OpPub, domain.Space, []byte(subject), domain.Space, []byte(reply), domain.CRLF, msg, domain.CRLF)
	slog.Debug(string(result))
	_, err := c.conn.Write(result)
	if err != nil {
		return fmt.Errorf("client PublishRequest, %v", err)
	}

	return nil
}
