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
	removeSubHandler(subscriberID int)
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

	logger.Info("close")
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
	logger.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return fmt.Errorf("client Publish, %v", err)
	}

	return nil
}

// Subscribe registers a handler that listen for messages sent to a subjects
// Uses SUB <subject> <sub.ID>
func (c *Conn) Subscribe(subject string, handler Handler) (int, error) {
	subscriberID := c.router.addSubHandler(handler)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, core.Space, []byte(subject), core.Space, subIDBytes, core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return -1, fmt.Errorf("client Subscribe, %v", err)
	}

	return subscriberID, nil
}

// Unsubscribe removes the handler from router and sends UNSUB to server
// Uses UNSUB <sub.ID>
func (c *Conn) Unsubscribe(subscriberID int) error {
	c.router.removeSubHandler(subscriberID)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpUnsub, core.Space, subIDBytes, core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return fmt.Errorf("client Unsubscribe, %v", err)
	}

	return nil
}

// QueueSubscribe as subscribe, but the server will randomly load balance among the handlers in the queue
// Uses SUB <subject> <sub.ID> <queue>
func (c *Conn) QueueSubscribe(subject string, queue string, handler Handler) (int, error) {
	subscriberID := c.router.addSubHandler(handler)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, core.Space, []byte(subject), core.Space, subIDBytes, core.Space, []byte(queue), core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
		return -1, fmt.Errorf("client QueueSubscribe, %v", err)
	}
	return subscriberID, nil
}

// Request as publish, but blocks until receives a response from a subscriber
// Creates a subscription to receive reply: SUB <subject> REPLY.<ID>
// Then publishes request PUB <subject> REPLY.<ID> \n\r message \n\r
func (c *Conn) Request(subject string, msg []byte) (*Message, error) {
	return c.RequestWithCtx(context.Background(), subject, msg)
}

func (c *Conn) RequestWithCtx(ctx context.Context, subject string, msg []byte) (*Message, error) {
	resCh := make(chan *Message)

	c.nextReply++
	reply := strconv.AppendInt([]byte("REPLY."), int64(c.nextReply), 10)

	subscriberID, err := c.Subscribe(string(reply), func(msg *Message) {
		logger.Debug("received", "msg", msg)
		resCh <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("client RequestWithCtx, %v", err)
	}

	result := bytes.Join([][]byte{core.OpPub, core.Space, []byte(subject), core.Space, reply, core.CRLF, msg, core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.conn.Write(result); err != nil {
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
		if err := c.Unsubscribe(subscriberID); err != nil {
			return nil, fmt.Errorf("client Request Unsubscribe, %v", err)
		}
		return r, nil
	}
}
