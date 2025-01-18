package domain

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

const TTL = 5 * time.Second

var (
	OpStop  = []byte{'S', 'T', 'O', 'P'}
	OpPub   = []byte{'P', 'U', 'B'}
	OpSub   = []byte{'S', 'U', 'B'}
	OpUnsub = []byte{'U', 'N', 'S', 'U', 'B'}
	OpPong  = []byte{'P', 'O', 'N', 'G'}
	OpPing  = []byte{'P', 'I', 'N', 'G'}

	OpOK  = []byte{'+', 'O', 'K'}
	OpERR = []byte{'-', 'E', 'R', 'R'}
	OpMsg = []byte{'M', 'S', 'G'}
)

var (
	CRLF     = []byte{'\r', '\n'}
	Space    = []byte{' '}
	Empty    []byte
	OK       = bytes.Join([][]byte{OpOK, CRLF}, nil)
	ControlC = []byte{255, 244, 255, 253, 6}
	Stop     = bytes.Join([][]byte{OpStop, CRLF}, nil)
	Ping     = bytes.Join([][]byte{OpPing, CRLF}, nil)
)

const CloseErr = "use of closed network connection"

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

		if !bytes.HasSuffix(buffer[:n], CRLF) && !Equals(buffer[:n], ControlC) {
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

func MonitorTimeout(c net.Conn, timeoutReset chan bool, stopTimeout chan bool, closeHandler chan bool) {
	timeout := time.NewTicker(TTL)
	timeoutCount := 0
Timeout:
	for {
		select {
		case <-timeoutReset:
			timeout.Reset(TTL)
			timeoutCount = 0

		case <-stopTimeout:
			break Timeout

		case <-timeout.C:
			timeoutCount++
			if timeoutCount > 2 {
				closeHandler <- true
				break Timeout
			}

			_, err := c.Write(bytes.Join([][]byte{OpPing, CRLF}, nil))
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					closeHandler <- true
					return
				}
			}
		}
	}
}

func Wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println()
}

func Equals(b1 []byte, b2 []byte) bool {
	return bytes.Compare(b1, b2) == 0
}
