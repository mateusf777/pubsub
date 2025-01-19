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

type router interface {
	route(msg *Message, subscriberID int) error
	addSubHandler(handler Handler) int
}

// Conn contains the state of the connection with the pubsub server to perform the necessary operations
type Conn struct {
	conn      net.Conn
	router    router
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

	ctx, cancel := context.WithCancel(context.Background())
	c := &Conn{
		conn:    conn,
		router:  newMsgRouter(),
		cancel:  cancel,
		drained: make(chan struct{}),
	}
	go c.handle(ctx)

	return c, nil
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

// Subscribe registers a handler that listen for messages sent to a subjects
// Uses command SUB <subject> <sub.ID>
func (c *Conn) Subscribe(subject string, handler Handler) error {
	subscriberID := c.router.addSubHandler(handler)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, []byte(subject), subIDBytes, core.CRLF}, nil)
	slog.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return fmt.Errorf("client Subscribe, %v", err)
	}

	return nil
}

// QueueSubscribe as subscribe, but the server will randomly load balance among the handlers in the queue
// Uses SUB <subject> <sub.ID> <queue>
func (c *Conn) QueueSubscribe(subject string, queue string, handle Handler) error {
	subscriberID := c.router.addSubHandler(handle)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, []byte(subject), subIDBytes, []byte("-1"), []byte(queue), core.CRLF}, nil)
	slog.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return fmt.Errorf("client QueueSubscribe, %v", err)
	}
	return nil
}

// Request as publish, but blocks until receives a response from a subscriber
// Creates a subscription to receive reply: SUB <subject> REPLY.<ID>
// Then publishes request PUB <subject> REPLY.<ID> \n\r message \n\r
func (c *Conn) Request(subject string, msg []byte) (*Message, error) {
	return c.RequestWithCtx(context.Background(), subject, msg)
}

func (c *Conn) RequestWithCtx(ctx context.Context, subject string, msg []byte) (*Message, error) {
	resCh := make(chan *Message)
	subscriberID := c.router.addSubHandler(func(msg *Message) {
		slog.Debug("received", "msg", msg)
		resCh <- msg
	})

	c.nextReply++
	reply := strconv.AppendInt([]byte("REPLY."), int64(c.nextReply), 10)
	slog.Debug(string(reply))

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, []byte(subject), reply, subIDBytes, core.CRLF}, nil)
	slog.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return nil, fmt.Errorf("client Request SUB, %v", err)
	}

	bResult := bytes.Join([][]byte{core.OpPub, core.Space, []byte(subject), core.Space, reply, core.CRLF, msg, core.CRLF}, nil)
	slog.Debug(string(bResult))

	if _, err := c.conn.Write(bResult); err != nil {
		return nil, fmt.Errorf("client Request PUB, %v", err)
	}

	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout")
	case r := <-resCh:
		return r, nil
	}
}
