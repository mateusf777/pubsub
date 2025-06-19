package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"time"
)

const (
	IdleTimeout = 60 * time.Second
	ClosedErr   = "use of closed network connection"
)

// logger is initialized with error level for the core package.
var logger *slog.Logger

type Info struct {
	ClientID string `json:"client_id"`
	Nonce    string `json:"nonce"`
}

func init() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	logger = slog.New(logHandler)
	logger = logger.With("lib", "core")
}

// SetLogLevel allows core user to configure a different level.
func SetLogLevel(level slog.Level) {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger = slog.New(logHandler)
	logger = logger.With("lib", "core")
}

// Protocol
var (
	// OpPub (PUB <subject> [reply_id] \r\n [msg] \r\n).
	// Publish a message to a subject with optional reply subject.
	// Client -> Server
	OpPub = []byte{'P', 'U', 'B'}

	// OpSub (SUB <subject> <sub_id> [group] \r\n).
	// Subscribe to a subject with optional group grouping.
	// Client -> Server
	OpSub = []byte{'S', 'U', 'B'}

	// OpUnsub (UNSUB <sub_id> \r\n)
	// Unsubscribes from a subject
	// Client -> Server
	OpUnsub = []byte{'U', 'N', 'S', 'U', 'B'}

	// OpStop (STOP \r\n)
	// Tells server to clean up connection.
	// Client -> Server
	OpStop = []byte{'S', 'T', 'O', 'P'}

	// OpPong (PONG \r\n)
	// Keep-alive response
	// Client -> Server
	OpPong = []byte{'P', 'O', 'N', 'G'}

	// OpInfo (INFO {"client_id":<clientID>, "nonce":<nonce>} \r\n).
	// Informs the clientID in the server and sends a nonce for authentication
	// Server -> Client
	OpInfo = []byte{'I', 'N', 'F', 'O'}

	// OpPing (PING \r\n)
	// Keep-alive message
	// Server -> Client
	OpPing = []byte{'P', 'I', 'N', 'G'}

	// OpMsg (MSG <subject> <sub_id> [reply-to] \r\n [payload] \r\n)
	// Delivers a message to a subscriber
	// Server -> Client
	OpMsg = []byte{'M', 'S', 'G'}

	// OpOK (+OK \r\n)
	// Acknowledges protocol messages.
	// Server -> Client
	OpOK = []byte{'+', 'O', 'K'}

	// OpERR (-ERR <error> \r\n)
	// Indicates protocol error.
	// Server -> Client
	OpERR = []byte{'-', 'E', 'R', 'R'}
)

// Helper values
var (
	Empty []byte

	CRLF     = []byte{'\r', '\n'}
	Space    = []byte{' '}
	OK       = BuildBytes(OpOK, CRLF)
	ControlC = []byte{255, 244, 255, 253, 6}
	Stop     = BuildBytes(OpStop, CRLF)
	Ping     = BuildBytes(OpPing, CRLF)
)

type ConnReader interface {
	Read()
}

type MsgProcessor interface {
	Process(ctx context.Context)
}

type KeepAliveEngine interface {
	Run(ctx context.Context)
}

type ConnectionHandlerConfig struct {
	Conn        net.Conn
	MsgHandler  MessageHandler
	IsClient    bool
	IdleTimeout time.Duration
}

type ConnectionHandler struct {
	conn         net.Conn
	reader       ConnReader
	msgProcessor MsgProcessor
	keepAlive    KeepAliveEngine
	isClient     bool
	data         chan []byte
	activity     chan struct{}
	close        chan struct{}
}

func NewConnectionHandler(cfg ConnectionHandlerConfig) (*ConnectionHandler, error) {

	if cfg.Conn == nil || cfg.MsgHandler == nil {
		return nil, errors.New("the attributes Conn and MsgHandler are required")
	}

	data := make(chan []byte)
	activity := make(chan struct{})
	closeHandler := make(chan struct{})

	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = IdleTimeout
	}

	reader := &ConnectionReader{
		reader:   cfg.Conn,
		buffer:   make([]byte, 1024),
		dataCh:   data,
		activity: activity,
	}

	msgProcessor := &MessageProcessor{
		writer:  cfg.Conn,
		handler: cfg.MsgHandler,
		remote:  cfg.Conn.RemoteAddr().String(),
		data:    data,
		close:   closeHandler,
	}

	keepAlive := &KeepAlive{
		writer:      cfg.Conn,
		remote:      cfg.Conn.RemoteAddr().String(),
		activity:    activity,
		close:       closeHandler,
		idleTimeout: cfg.IdleTimeout,
	}

	return &ConnectionHandler{
		conn:         cfg.Conn,
		reader:       reader,
		msgProcessor: msgProcessor,
		keepAlive:    keepAlive,
		isClient:     cfg.IsClient,
		data:         data,
		activity:     activity,
		close:        closeHandler,
	}, nil
}

func (ch *ConnectionHandler) Handle() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ch.reader.Read()

	go ch.msgProcessor.Process(ctx)

	go ch.keepAlive.Run(ctx)

	<-ch.close
}

func (ch *ConnectionHandler) Close() {
	l := logger.With("location", "ConnectionHandler.Close()")

	remote := ch.conn.RemoteAddr().String()
	l.Info("Closing Connection", "remote", remote)

	if ch.isClient {
		_, err := ch.conn.Write(Stop)
		if err != nil {
			l.Info("Failed to send STOP to server, will proceed to close", "error", err)
		} else {
			l.Info("Waiting for Server to close the connection")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			ch.waitServerClose(ctx)
			l.Info("Server closed the connection")
		}
	}

	if err := ch.conn.Close(); err != nil {
		l.Warn("conn.Close()", "error", err)
	}

	close(ch.close)
	close(ch.activity)
	close(ch.data)

	l.Info("Close Connection", "remote", remote)
}

// waitServerClose waits the connection to be closed by the server
// This assures all messages were consumed and processed.
func (ch *ConnectionHandler) waitServerClose(ctx context.Context) {
	l := logger.With("location", "Conn.waitServerClose()")

	ticker := time.NewTicker(250 * time.Millisecond)
	buf := make([]byte, 8)
loop:
	for {
		select {
		case <-ticker.C:
			_, err := ch.conn.Read(buf)
			if err == nil {
				l.Debug("Not disconnected yet")
				continue
			}
			l.Debug("Read", "error", err)
			break loop
		case <-ctx.Done():
			l.Debug("Done")
			break loop
		}
	}

	l.Info("Server closed connection")
}

// ConnectionReader implements Read.
type ConnectionReader struct {
	reader   io.Reader
	buffer   []byte
	dataCh   chan<- []byte
	activity chan<- struct{}
}

// Read connection stream, adds data to buffer, split messages and send them to the channel.
func (cr *ConnectionReader) Read() {
	l := logger.With("location", "ConnectionReader.Read()")
	accumulator := Empty
	for {
		n, err := cr.reader.Read(cr.buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				l.Info("Connection closed")
				break
			}
			l.Info("net.Conn Read", "error", err)
			break
		}
		cr.activity <- struct{}{}

		l.Debug("Read", "data", string(cr.buffer[:n]))

		toBeSplit := BuildBytes(accumulator, cr.buffer[:n])
		messages := bytes.Split(toBeSplit, CRLF)
		accumulator = Empty

		if !bytes.HasSuffix(cr.buffer[:n], CRLF) && !bytes.Equal(cr.buffer[:n], ControlC) {
			accumulator = messages[len(messages)-1]
			messages = messages[:len(messages)-1]
		}

		if len(messages) > 0 && len(messages[len(messages)-1]) == 0 {
			messages = messages[:len(messages)-1]
		}

		l.Debug("messages", "messages", messages)
		for _, msg := range messages {
			l.Debug("Send msg to dataCh", "msg", string(msg))
			cr.dataCh <- msg
		}
	}
}

type MessageHandler func(writer io.Writer, data []byte, dataCh <-chan []byte, close chan<- struct{})

type MessageProcessor struct {
	writer  io.Writer
	handler MessageHandler
	remote  string
	data    <-chan []byte
	close   chan<- struct{}
}

func (m *MessageProcessor) Process(ctx context.Context) {
	l := logger.With("location", "MessageProcessor.Process()")
	for {
		select {
		case <-ctx.Done():
			l.Info("MessageProcessor Done", "remote", m.remote)
			return
		case data, ok := <-m.data:
			if !ok {
				l.Info("MessageProcessor data channel closed", "remote", m.remote)
				return
			}
			m.handler(m.writer, data, m.data, m.close)
		}
	}
}

// KeepAlive can run a keep-alive mechanism for a connection between PubSub server and client.
type KeepAlive struct {
	writer      io.Writer
	remote      string
	activity    <-chan struct{}
	close       chan<- struct{}
	idleTimeout time.Duration
}

// Run KeepAlive mechanism. Normal traffic resets idle timeout. It sends PING to remote if idle timeout happens.
// After two pings without response, sends signal to close connection.
func (k *KeepAlive) Run(ctx context.Context) {
	l := logger.With("location", "KeepAlive.Run()")
	checkTimeout := time.NewTicker(k.idleTimeout)
	defer checkTimeout.Stop()

	count := 0
loop:
	for {
		select {
		case _, ok := <-k.activity:
			if !ok {
				l.Debug("KeepAlive activity channel closed", "remote", k.remote)
				break loop
			}
			l.Debug("Reset", "remote", k.remote)
			checkTimeout.Reset(k.idleTimeout)
			count = 0

		case <-ctx.Done():
			l.Debug("Done", "remote", k.remote)
			break loop

		case <-checkTimeout.C:
			count++
			l.Debug("Check", "remote", k.remote, "count", count)
			if count > 2 {
				k.close <- struct{}{}
				break loop
			}

			l.Debug("Send PING", "remote", k.remote)
			_, err := k.writer.Write(Ping)
			if err != nil {
				l.Info("net.Conn Write, closing...", "error", err)
				k.close <- struct{}{}
				return
			}
		}
	}
	l.Info("Closed")
}

// BuildBytes helps create a slice of bytes from multiple slices of bytes.
func BuildBytes(b ...[]byte) []byte {
	return bytes.Join(b, nil)
}
