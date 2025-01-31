package client

import (
	"errors"
	"github.com/mateusf777/pubsub/core"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestMessageHandler(t *testing.T) {
	type args struct {
		router func(m *Mockrouter)
		reader func(r *io.PipeReader)
		dataCh func(d chan []byte)
		raw    []byte
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "MSG",
			args: args{
				router: func(m *Mockrouter) {
					m.On("route",
						&Message{
							Subject: "test",
							Data:    []byte("test"),
						}, 1,
					).Return(nil).Once()
				},
				dataCh: func(d chan []byte) {
					d <- []byte("test")
				},
				raw: core.BuildBytes(core.OpMsg, core.Space, []byte("test"), core.Space, []byte("1")),
			},
		},
		{
			name: "MSG with reply",
			args: args{
				router: func(m *Mockrouter) {
					m.On("route",
						&Message{
							Subject: "test",
							Reply:   "test",
							Data:    []byte("test"),
						}, 1,
					).Return(nil).Once()
				},
				dataCh: func(d chan []byte) {
					d <- []byte("test")
				},
				raw: core.BuildBytes(core.OpMsg, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte("test")),
			},
		},
		{
			name: "MSG invalid format",
			args: args{
				dataCh: func(d chan []byte) {
					d <- []byte("test")
				},
				raw: core.OpMsg,
			},
		},
		{
			name: "MSG router error",
			args: args{
				router: func(m *Mockrouter) {
					m.On("route",
						&Message{
							Subject: "test",
							Data:    []byte("test"),
						}, 1,
					).Return(errors.New("error")).Once()
				},
				dataCh: func(d chan []byte) {
					d <- []byte("test")
				},
				raw: core.BuildBytes(core.OpMsg, core.Space, []byte("test"), core.Space, []byte("1")),
			},
		},
		{
			name: "PING",
			args: args{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpPong, core.CRLF), buf[:n])
				},
				raw: core.OpPing,
			},
		},
		{
			name: "PING with write error",
			args: args{
				reader: func(r *io.PipeReader) {
					_ = r.Close()
				},
				raw: core.OpPing,
			},
		},
		{
			name: "PING with write broken pipe",
			args: args{
				reader: func(r *io.PipeReader) {
					_ = r.CloseWithError(errors.New("broken pipe"))
				},
				raw: core.OpPing,
			},
		},
		{
			name: "PONG",
			args: args{
				raw: core.OpPong,
			},
		},
		{
			name: "OK",
			args: args{
				raw: core.OpOK,
			},
		},
		{
			name: "ERR",
			args: args{
				raw: core.OpERR,
			},
		},
		{
			name: "Empty",
			args: args{
				raw: core.Empty,
			},
		},
		{
			name: "UNKNOWN",
			args: args{
				raw: []byte("UNKNOWN"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()
			mockRouter := NewMockrouter(t)
			dataCh := make(chan []byte)

			if tt.args.router != nil {
				tt.args.router(mockRouter)
			}

			if tt.args.reader != nil {
				go tt.args.reader(testReader)
			}

			if tt.args.dataCh != nil {
				go tt.args.dataCh(dataCh)
			}

			handler := MessageHandler(mockRouter)
			handler(testWriter, tt.args.raw, dataCh, nil)
		})
	}
}
