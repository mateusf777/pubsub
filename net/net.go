package net

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mateusf777/pubsub/log"
)

const TTL = 5 * time.Minute

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
	CRLF  = []byte{'\r', '\n'}
	Space = []byte{' '}
	Empty []byte
	OK    = bytes.Join([][]byte{OpOK, CRLF}, []byte{})
)

const CloseErr = "use of closed network connection"

func Read(c net.Conn, buffer []byte, dataCh chan []byte) {
	accumulator := Empty
	for {
		n, err := c.Read(buffer)
		if err != nil {
			return
		}

		toBeSplit := bytes.Join([][]byte{accumulator, buffer[:n]}, []byte{})
		messages := bytes.Split(toBeSplit, CRLF)
		accumulator = Empty

		if !bytes.HasSuffix(buffer[:n], CRLF) {
			accumulator = messages[len(messages)-1]
			messages = messages[:len(messages)-1]
		}

		for _, msg := range messages {
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
			log.Debug("Stop timeout process %s\n", c.RemoteAddr().String())
			break Timeout

		case <-timeout.C:
			timeoutCount++
			if timeoutCount > 2 {
				log.Info("Timeout %s\n", c.RemoteAddr().String())
				closeHandler <- true
				break Timeout
			}

			_, err := c.Write(bytes.Join([][]byte{OpPing, CRLF}, []byte{}))
			if err != nil {
				log.Error("%v\n", err)
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
