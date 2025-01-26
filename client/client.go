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

type router interface {
	route(msg *Message, subscriberID int) error
	addSubHandler(handler Handler) int
	removeSubHandler(subscriberID int)
}

// Conn contains the state of the connection with the pubsub server to perform the necessary operations
type Conn struct {
	conn         net.Conn
	router       router
	connReader   *core.ConnectionReader
	msgProc      *messageProcessor
	keepAlive    *core.KeepAlive
	client       *Client
	closeHandler chan bool
	cancel       context.CancelFunc
	drained      chan struct{}
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
func Connect(address string) (*Conn, error) {
	logger.With("location", "Connect()").Debug("Connect")

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	dataCh := make(chan []byte, 10)
	resetInactivity := make(chan bool)
	stopKeepAlive := make(chan bool)
	closeHandler := make(chan bool)

	rt := newMsgRouter()

	connReader, err := core.NewConnectionReader(core.ConnectionReaderConfig{
		Reader:   conn,
		DataChan: dataCh,
	})
	if err != nil {
		return nil, err
	}

	client := &Client{
		writer: conn,
		router: rt,
	}

	msgProc := &messageProcessor{
		writer:          conn,
		router:          rt,
		client:          client,
		data:            dataCh,
		resetInactivity: resetInactivity,
		stopKeepAlive:   stopKeepAlive,
		closeHandler:    closeHandler,
	}

	keepAlive, err := core.NewKeepAlive(core.KeepAliveConfig{
		Writer:          conn,
		Client:          conn.RemoteAddr().String(),
		ResetInactivity: resetInactivity,
		StopKeepAlive:   stopKeepAlive,
		CloseHandler:    closeHandler,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Conn{
		conn:         conn,
		router:       rt,
		connReader:   connReader,
		msgProc:      msgProc,
		keepAlive:    keepAlive,
		client:       client,
		closeHandler: closeHandler,
		cancel:       cancel,
		drained:      make(chan struct{}),
	}

	go c.handle(ctx)

	return c, nil
}

// Close sends the "stop" operation so the server can clean up the resources
func (c *Conn) Close() {
	l := logger.With("location", "Conn.Close()")
	l.Debug("Close")

	_, err := c.conn.Write(core.Stop)
	if err != nil {
		l.Info("Failed to send message to server, will proceed with close", "error", err)
	} else {
		l.Info("Waiting for Server to close the connection")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		c.waitServerClose(ctx)
		l.Info("Server closed the connection")
	}

	l.Info("Close reader")
	c.cancel()
	l.Info("Waiting for reader to close")
	<-c.drained
}

// waitServerClose waits the connection to be closed by the server
// This assures all messages were consumed and processed.
func (c *Conn) waitServerClose(ctx context.Context) {
	l := logger.With("location", "Conn.waitServerClose()")

	ticker := time.NewTimer(100 * time.Millisecond)

	buf := make([]byte, 8)
loop:
	for {
		select {
		case <-ticker.C:
			_, err := c.conn.Read(buf)
			if err == nil {
				continue
			}
			l.Info("Read", "error", err)
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	l.Info("Server closed connection")
}

func (c *Conn) GetClient() *Client {
	return c.client
}

type Client struct {
	writer    io.Writer
	router    router
	nextReply int
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
