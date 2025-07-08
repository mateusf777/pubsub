package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

const (
	IdleTimeout = 60 * time.Second                   // Default idle timeout for connections in the core package.
	ClosedErr   = "use of closed network connection" // Error string used to identify closed network connections.
)

// logger is initialized with error level for the core package.
// Used throughout the core package for structured logging.
var logger *slog.Logger

// Info holds client identification and authentication nonce information.
// Used in the INFO protocol message.
type Info struct {
	ClientID string `json:"client_id"` // Unique identifier for the client.
	Nonce    string `json:"nonce"`     // Nonce for authentication.
}

func init() {
	// Initializes the logger with error level and attaches the "core" library label.
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	logger = slog.New(logHandler)
	logger = logger.With("lib", "core")
}

// SetLogLevel allows core user to configure a different level.
// Dynamically sets the log level for the core logger.
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
	// Used to identify PUB commands in the protocol.
	OpPub = []byte{'P', 'U', 'B'}

	// OpSub (SUB <subject> <sub_id> [group] \r\n).
	// Subscribe to a subject with optional group grouping.
	// Client -> Server
	// Used to identify SUB commands in the protocol.
	OpSub = []byte{'S', 'U', 'B'}

	// OpUnsub (UNSUB <sub_id> \r\n)
	// Unsubscribes from a subject
	// Client -> Server
	// Used to identify UNSUB commands in the protocol.
	OpUnsub = []byte{'U', 'N', 'S', 'U', 'B'}

	// OpStop (STOP \r\n)
	// Tells server to clean up connection.
	// Client -> Server
	// Used to identify STOP commands in the protocol.
	OpStop = []byte{'S', 'T', 'O', 'P'}

	// OpPong (PONG \r\n)
	// Keep-alive response
	// Client -> Server
	// Used to identify PONG responses in the protocol.
	OpPong = []byte{'P', 'O', 'N', 'G'}

	// OpInfo (INFO {"client_id":<clientID>, "nonce":<nonce>} \r\n).
	// Informs the clientID in the server and sends a nonce for authentication
	// Server -> Client
	// Used to identify INFO messages in the protocol.
	OpInfo = []byte{'I', 'N', 'F', 'O'}

	// OpPing (PING \r\n)
	// Keep-alive message
	// Server -> Client
	// Used to identify PING messages in the protocol.
	OpPing = []byte{'P', 'I', 'N', 'G'}

	// OpMsg (MSG <subject> <sub_id> [reply-to] \r\n [payload] \r\n)
	// Delivers a message to a subscriber
	// Server -> Client
	// Used to identify MSG messages in the protocol.
	OpMsg = []byte{'M', 'S', 'G'}

	// OpOK (+OK \r\n)
	// Acknowledges protocol messages.
	// Server -> Client
	// Used to identify OK responses in the protocol.
	OpOK = []byte{'+', 'O', 'K'}

	// OpERR (-ERR <error> \r\n)
	// Indicates protocol error.
	// Server -> Client
	// Used to identify ERR responses in the protocol.
	OpERR = []byte{'-', 'E', 'R', 'R'}
)

// Helper values
var (
	Empty []byte // Represents an empty byte slice.

	CRLF     = []byte{'\r', '\n'}            // Carriage return and line feed.
	Space    = []byte{' '}                   // Space character.
	OK       = BuildBytes(OpOK, CRLF)        // Prebuilt OK response.
	ControlC = []byte{255, 244, 255, 253, 6} // Control-C sequence for telnet.
	Stop     = BuildBytes(OpStop, CRLF)      // Prebuilt STOP command.
	Ping     = BuildBytes(OpPing, CRLF)      // Prebuilt PING command.
)

// ConnReader abstracts reading from a connection and sending data to a channel.
type ConnReader interface {
	Read(ctx context.Context)
}

// MsgProcessor abstracts processing messages from a channel.
type MsgProcessor interface {
	Process(ctx context.Context)
}

// KeepAliveEngine abstracts running a keep-alive mechanism.
type KeepAliveEngine interface {
	Run(ctx context.Context)
}

// ConnectionHandlerConfig holds configuration for creating a ConnectionHandler.
type ConnectionHandlerConfig struct {
	Conn         net.Conn       // The network connection.
	MsgHandler   MessageHandler // Handler for incoming messages.
	IsClient     bool           // True if this is a client connection.
	IdleTimeout  time.Duration  // Idle timeout for keep-alive.
	CloseTimeout time.Duration  // Timeout for closing the connection.
}

// ConnectionHandler manages the lifecycle and coordination of a connection,
// including reading, message processing, and keep-alive.
type ConnectionHandler struct {
	conn         net.Conn
	reader       ConnReader
	msgProcessor MsgProcessor
	keepAlive    KeepAliveEngine
	isClient     bool
	data         chan []byte
	activity     chan struct{}
	close        chan struct{}
	closeTimeout time.Duration
}

// NewConnectionHandler creates and initializes a new ConnectionHandler
// with the provided configuration.
func NewConnectionHandler(cfg ConnectionHandlerConfig) (*ConnectionHandler, error) {

	if cfg.Conn == nil || cfg.MsgHandler == nil {
		return nil, errors.New("the attributes Conn and MsgHandler are required")
	}

	data := make(chan []byte, 256)      // Channel for incoming data.
	activity := make(chan struct{})     // Channel for activity signaling.
	closeHandler := make(chan struct{}) // Channel for signaling closure.

	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = IdleTimeout
	}

	if cfg.CloseTimeout <= 0 {
		cfg.CloseTimeout = time.Minute
	}

	reader := &ConnectionReader{
		reader:   cfg.Conn,
		buffer:   make([]byte, 16*1024),
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
		closeTimeout: cfg.CloseTimeout,
	}, nil
}

// Handle starts the connection's reader, message processor, and keep-alive goroutines,
// and blocks until the connection is closed.
func (ch *ConnectionHandler) Handle() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ch.reader.Read(ctx)

	go ch.msgProcessor.Process(ctx)

	go ch.keepAlive.Run(ctx)

	<-ch.close // Wait for close signal.
}

// Close gracefully closes the connection, optionally waiting for the server to close first
// if this is a client connection.
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
			ctx, cancel := context.WithTimeout(context.Background(), ch.closeTimeout)
			defer cancel()

			ch.waitServerClose(ctx)
			l.Info("Server closed the connection")
		}
	}

	if err := ch.conn.Close(); err != nil {
		l.Warn("conn.Close()", "error", err)
	}

	l.Info("Close Connection", "remote", remote)
}

// waitServerClose waits the connection to be closed by the server
// This assures all messages were consumed and processed.
// Used only for client connections to ensure graceful shutdown.
func (ch *ConnectionHandler) waitServerClose(ctx context.Context) {
	l := logger.With("location", "ConnectionHandler.waitServerClose()")

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
// Reads from the network connection, splits messages, and sends them to dataCh.
type ConnectionReader struct {
	reader   io.Reader       // The underlying reader (usually net.Conn).
	buffer   []byte          // Buffer for reading data.
	dataCh   chan<- []byte   // Channel to send parsed messages.
	activity chan<- struct{} // Channel to signal activity for keep-alive.
}

// bufferPool is used to reuse buffers for message accumulation and reduce allocations.
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 16*1024)
	},
}

// Read connection stream, adds data to buffer, split messages and send them to the channel.
// Handles context cancellation and connection closure.
func (cr *ConnectionReader) Read(ctx context.Context) {
	defer close(cr.dataCh)
	defer close(cr.activity)

	l := logger.With("location", "ConnectionReader.Read()")

	var accumulator []byte
	for {
		select {
		case <-ctx.Done():
			l.Info("Context canceled, stopping reader")
			return
		default:

		}

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
		accumulator = nil

		if !bytes.HasSuffix(cr.buffer[:n], CRLF) && !bytes.Equal(cr.buffer[:n], ControlC) {
			accumulator = bufferPool.Get().([]byte)
			accumulator = accumulator[:len(messages[len(messages)-1])]
			copy(accumulator, messages[len(messages)-1])

			messages = messages[:len(messages)-1]
		}

		if len(messages) > 0 && len(messages[len(messages)-1]) == 0 {
			messages = messages[:len(messages)-1]
		}

		//l.Debug("messages", "messages", messages)
		for _, msg := range messages {
			select {
			case <-ctx.Done():
				l.Info("Context canceled while sending message")
				return
			case cr.dataCh <- msg:
				if l.Enabled(ctx, slog.LevelDebug) {
					l.Debug("Send msg to dataCh", "msg", string(msg))
				}
			}
		}
	}
}

// MessageHandler defines the signature for message processing callbacks.
// Used by MessageProcessor to handle incoming messages.
type MessageHandler func(writer io.Writer, data []byte, dataCh <-chan []byte, close chan<- struct{})

// MessageProcessor processes messages from the data channel using the provided handler.
type MessageProcessor struct {
	writer  io.Writer       // Writer to send responses.
	handler MessageHandler  // Handler function for processing messages.
	remote  string          // Remote address for logging.
	data    <-chan []byte   // Channel to receive messages.
	close   chan<- struct{} // Channel to signal closure.
}

// Process reads messages from the data channel and invokes the handler.
// Exits when the context is canceled or the data channel is closed.
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
// Monitors activity and sends PINGs if the connection is idle.
type KeepAlive struct {
	writer      io.Writer       // Writer to send PINGs.
	remote      string          // Remote address for logging.
	activity    <-chan struct{} // Channel to receive activity signals.
	close       chan<- struct{} // Channel to signal closure.
	idleTimeout time.Duration   // Idle timeout duration.
}

// Run KeepAlive mechanism. Normal traffic resets idle timeout. It sends PING to remote if idle timeout happens.
// After two pings without response, sends signal to close connection.
// Exits when the context is canceled or the activity channel is closed.
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
// Used for constructing protocol messages efficiently.
func BuildBytes(b ...[]byte) []byte {
	return bytes.Join(b, nil)
}
