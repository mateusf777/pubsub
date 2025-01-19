package core

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const IdleTimeout = 5 * time.Second

// Protocol
var (
	// OpPub (PUB <subject> [reply_id] \n\r [msg] \n\r).
	// Publish a message to a subject with optional reply subject.
	// Client -> Server
	OpPub = []byte{'P', 'U', 'B'}

	// OpSub (SUB <subject> <sub_id> [queue] \n\r).
	// Subscribe to a subject with optional queue grouping.
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
	OK       = bytes.Join([][]byte{OpOK, CRLF}, nil)
	ControlC = []byte{255, 244, 255, 253, 6}
	Stop     = bytes.Join([][]byte{OpStop, CRLF}, nil)
	Ping     = bytes.Join([][]byte{OpPing, CRLF}, nil)
)

const CloseErr = "use of closed network connection"

// Read connection stream, adds data to buffer, split messages and send them to the channel.
func Read(c net.Conn, buffer []byte, dataCh chan []byte) {
	accumulator := Empty
	for {
		n, err := c.Read(buffer)
		if err != nil {
			return
		}

		toBeSplit := bytes.Join([][]byte{accumulator, buffer[:n]}, nil)
		messages := bytes.Split(toBeSplit, CRLF)
		accumulator = Empty

		if !bytes.HasSuffix(buffer[:n], CRLF) && !bytes.Equal(buffer[:n], ControlC) {
			accumulator = messages[len(messages)-1]
			messages = messages[:len(messages)-1]
		}

		if len(messages) > 0 && len(messages[len(messages)-1]) == 0 {
			messages = messages[:len(messages)-1]
		}

		for _, msg := range messages {
			logger.Info(string(msg))
			dataCh <- msg
		}
	}
}

// KeepAlive mechanism. Normal traffic resets idle timeout. It sends PING to client if idle timeout happens.
// After two pings without response, sends signal to close connection.
func KeepAlive(c net.Conn, reset chan bool, stop chan bool, close chan bool) {
	checkTicket := time.NewTicker(IdleTimeout)
	count := 0
active:
	for {
		select {
		case <-reset:
			checkTicket.Reset(IdleTimeout)
			count = 0

		case <-stop:
			break active

		case <-checkTicket.C:
			count++
			if count > 2 {
				close <- true
				break active
			}

			_, err := c.Write(bytes.Join([][]byte{OpPing, CRLF}, nil))
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					close <- true
					return
				}
			}
		}
	}
}

// Wait for system signal (SIGINT, SIGTERM)
func Wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println()
}
