package core

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Reader interface {
	io.Reader
}

type Writer interface {
	io.Writer
}

const (
	IdleTimeout = 5 * time.Second
	ClosedErr   = "use of closed network connection"
)

// logger is initialized with error level for the core package.
var logger *slog.Logger

func init() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	logger = slog.New(logHandler)
}

// SetLogLevel allows core user to configure a different level.
func SetLogLevel(level slog.Level) {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger = slog.New(logHandler)
}

// Protocol
var (
	// OpPub (PUB <subject> [reply_id] \n\r [msg] \n\r).
	// Publish a message to a subject with optional reply subject.
	// Client -> Server
	OpPub = []byte{'P', 'U', 'B'}

	// OpSub (SUB <subject> <sub_id> [group] \n\r).
	// Subscribe to a subject with optional group grouping.
	// Client -> Server
	OpSub = []byte{'S', 'U', 'B'}

	// OpUnsub (UNSUB <sub_id> \n\r)
	// Unsubscribes from a subject
	// Client -> Server
	OpUnsub = []byte{'U', 'N', 'S', 'U', 'B'}

	// OpStop (STOP \n\r)
	// Tells server to clean up connection.
	// Client -> Server
	OpStop = []byte{'S', 'T', 'O', 'P'}

	// OpPong (PONG \n\r)
	// Keep-alive response
	// Client -> Server
	OpPong = []byte{'P', 'O', 'N', 'G'}

	// OpPing (PING \n\r)
	// Keep-alive message
	// Server -> Client
	OpPing = []byte{'P', 'I', 'N', 'G'}

	// OpMsg (MSG <subject> <sub_id> [reply-to] \n\r [payload] \n\r)
	// Delivers a message to a subscriber
	// Server -> Client
	OpMsg = []byte{'M', 'S', 'G'}

	// OpOK (+OK \n\r)
	// Acknowledges protocol messages.
	// Server -> Client
	OpOK = []byte{'+', 'O', 'K'}

	// OpERR (-ERR <error> \n\r)
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

// ConnectionReaderConfig is used to create a ConnectionReader.
type ConnectionReaderConfig struct {
	Reader     Reader
	BufferSize int
	DataChan   chan []byte
}

// ConnectionReader implements Read.
type ConnectionReader struct {
	reader Reader
	buffer []byte
	dataCh chan []byte
}

// NewConnectionReader creates a ConnectionReader from configuration.
// Conn and DataChan are required. BufferSize will use 1024 if a value <= 0 is passed.
func NewConnectionReader(cfg ConnectionReaderConfig) (*ConnectionReader, error) {
	if cfg.Reader == nil || cfg.DataChan == nil {
		return nil, errors.New("required configuration not set")
	}

	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1024
	}

	return &ConnectionReader{
		reader: cfg.Reader,
		buffer: make([]byte, cfg.BufferSize),
		dataCh: cfg.DataChan,
	}, nil
}

// Read connection stream, adds data to buffer, split messages and send them to the channel.
func (cr *ConnectionReader) Read() {
	accumulator := Empty
	for {
		n, err := cr.reader.Read(cr.buffer)
		if err != nil {
			if strings.Contains(err.Error(), ClosedErr) {
				logger.Debug("Connection closed")
				return
			}
			logger.Debug("net.Conn Read", "error", err)
			return
		}

		logger.Debug("Read", "data", string(cr.buffer[:n]))

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

		for _, msg := range messages {
			logger.Debug(string(msg))
			cr.dataCh <- msg
		}
	}
}

// KeepAliveConfig configuration for the keep-alive mechanism
type KeepAliveConfig struct {
	Writer      Writer
	Client      string
	ResetCh     chan bool
	StopCh      chan bool
	CloseCh     chan bool
	IdleTimeout time.Duration
}

// KeepAlive can run a keep-alive mechanism for a connection between PubSub server and client.
type KeepAlive struct {
	writer      Writer
	client      string
	reset       chan bool
	stop        chan bool
	close       chan bool
	idleTimeout time.Duration
}

// NewKeepAlive from configuration
func NewKeepAlive(cfg KeepAliveConfig) (*KeepAlive, error) {
	if cfg.Writer == nil || len(cfg.Client) == 0 || cfg.ResetCh == nil || cfg.StopCh == nil || cfg.CloseCh == nil {
		return nil, errors.New("required configuration not set")
	}

	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = IdleTimeout
	}

	return &KeepAlive{
		writer:      cfg.Writer,
		client:      cfg.Client,
		reset:       cfg.ResetCh,
		stop:        cfg.StopCh,
		close:       cfg.CloseCh,
		idleTimeout: cfg.IdleTimeout,
	}, nil
}

// Run KeepAlive mechanism. Normal traffic resets idle timeout. It sends PING to client if idle timeout happens.
// After two pings without response, sends signal to close connection.
func (k *KeepAlive) Run() {
	checkTimeout := time.NewTicker(k.idleTimeout)
	count := 0
active:
	for {
		select {
		case <-k.reset:
			logger.Debug("keep alive reset", "client", k.client)
			checkTimeout.Reset(k.idleTimeout)
			count = 0

		case <-k.stop:
			logger.Debug("keep alive stop", "client", k.client)
			break active

		case <-checkTimeout.C:
			count++
			logger.Debug("keep alive check", "client", k.client, "count", count)
			if count > 2 {
				k.close <- true
				break active
			}

			logger.Debug("keep alive PING", "client", k.client)
			_, err := k.writer.Write(BuildBytes(OpPing, CRLF))
			if err != nil {
				logger.Error("net.Conn Write", "error", err)
				if strings.Contains(err.Error(), "broken pipe") {
					k.close <- true
					return
				}
			}
		}
	}
}

// BuildBytes helps create a slice of bytes from multiple slices of bytes.
func BuildBytes(b ...[]byte) []byte {
	return bytes.Join(b, nil)
}
