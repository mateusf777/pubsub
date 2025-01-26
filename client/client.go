package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/mateusf777/pubsub/core"
)

// Message contains data and metadata about a message sent from a publisher to a subscriber
type Message struct {
	Subject string
	Reply   string
	Data    []byte
}

type router interface {
	route(msg *Message, subscriberID int) error
	addSubHandler(handler Handler) int
	removeSubHandler(subscriberID int)
}

var logger *slog.Logger

func SetLogLevel(level slog.Level) {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger = slog.New(logHandler)
	logger = logger.With("lib", "client")
}

func init() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	logger = slog.New(logHandler)
	logger = logger.With("lib", "client")
}

// Connect makes the connection with the server
func Connect(address string) (*Client, error) {
	logger.With("location", "Connect()").Debug("Connect")

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	rt := newMsgRouter()

	connHandler := core.NewConnectionHandler(core.ConnectionHandlerConfig{
		Conn:       conn,
		MsgHandler: MessageHandler(rt),
		IsClient:   true,
	})

	client := &Client{
		connHandler: connHandler,
		writer:      conn,
		router:      rt,
	}

	go connHandler.Handle()

	return client, nil
}

type Client struct {
	connHandler *core.ConnectionHandler
	writer      io.Writer
	router      router
	nextReply   int
}

func (c *Client) Close() {
	c.connHandler.Close()
}

// Publish sends a message for a subject
// Uses command PUB <subject> \n\r message \n\r
func (c *Client) Publish(subject string, msg []byte) error {
	result := bytes.Join([][]byte{core.OpPub, core.Space, []byte(subject), core.CRLF, msg, core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.writer.Write(result); err != nil {
		return fmt.Errorf("client Publish, %v", err)
	}

	return nil
}

// Subscribe registers a handler that listen for messages sent to a subjects
// Uses SUB <subject> <sub.ID>
func (c *Client) Subscribe(subject string, handler Handler) (int, error) {
	subscriberID := c.router.addSubHandler(handler)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, core.Space, []byte(subject), core.Space, subIDBytes, core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.writer.Write(result); err != nil {
		return -1, fmt.Errorf("client Subscribe, %v", err)
	}

	return subscriberID, nil
}

// Unsubscribe removes the handler from router and sends UNSUB to server
// Uses UNSUB <sub.ID>
func (c *Client) Unsubscribe(subscriberID int) error {
	c.router.removeSubHandler(subscriberID)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpUnsub, core.Space, subIDBytes, core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.writer.Write(result); err != nil {
		return fmt.Errorf("client Unsubscribe, %v", err)
	}

	return nil
}

// QueueSubscribe as subscribe, but the server will randomly load balance among the handlers in the queue
// Uses SUB <subject> <sub.ID> <queue>
func (c *Client) QueueSubscribe(subject string, queue string, handler Handler) (int, error) {
	subscriberID := c.router.addSubHandler(handler)

	subIDBytes := strconv.AppendInt([]byte{}, int64(subscriberID), 10)

	result := bytes.Join([][]byte{core.OpSub, core.Space, []byte(subject), core.Space, subIDBytes, core.Space, []byte(queue), core.CRLF}, nil)
	logger.Debug(string(result))

	if _, err := c.writer.Write(result); err != nil {
		return -1, fmt.Errorf("client QueueSubscribe, %v", err)
	}
	return subscriberID, nil
}

// Request as publish, but blocks until receives a response from a subscriber
// Creates a subscription to receive reply: SUB <subject> REPLY.<ID>
// Then publishes request PUB <subject> REPLY.<ID> \n\r message \n\r
func (c *Client) Request(subject string, msg []byte) (*Message, error) {
	return c.RequestWithCtx(context.Background(), subject, msg)
}

func (c *Client) RequestWithCtx(ctx context.Context, subject string, msg []byte) (*Message, error) {
	l := logger.With("location", "Client.RequestWithCtx()")
	l.Debug("RequestWithCtx")

	resCh := make(chan *Message)

	c.nextReply++
	reply := strconv.AppendInt([]byte("REPLY."), int64(c.nextReply), 10)

	subscriberID, err := c.Subscribe(string(reply), func(msg *Message) {
		l.Debug("received", "msg", msg)
		resCh <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("client RequestWithCtx, %v", err)
	}

	result := bytes.Join([][]byte{core.OpPub, core.Space, []byte(subject), core.Space, reply, core.CRLF, msg, core.CRLF}, nil)
	l.Debug(string(result))

	if _, err := c.writer.Write(result); err != nil {
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

func (c *Client) Reply(msg *Message, data []byte) error {
	if len(msg.Reply) == 0 {
		return nil
	}
	return c.Publish(msg.Reply, data)
}
