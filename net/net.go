package net

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mateusf777/pubsub/log"
)

const TTL = 5 * time.Minute

const (
	OpStop  = "STOP"
	OpPub   = "PUB"
	OpSub   = "SUB"
	OpUnsub = "UNSUB"
	OpPong  = "PONG"
	OpPing  = "PING"

	OpOK  = "+OK"
	OpERR = "-ERR"
	OpMsg = "MSG"
)

const (
	CRLF  = "\r\n"
	Space = " "
	Empty = ""
	OK    = OpOK + CRLF
)

const CloseErr = "use of closed network connection"

func Read(c net.Conn, buffer []byte, dataCh chan string) {
	accumulator := Empty
	for {
		n, err := c.Read(buffer)
		if err != nil {
			return
		}

		messages := strings.Split(accumulator+string(buffer[:n]), CRLF)
		accumulator = Empty

		if !strings.HasSuffix(string(buffer[:n]), CRLF) {
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

			_, err := c.Write([]byte(OpPing + CRLF))
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
