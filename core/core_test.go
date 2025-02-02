package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var expectedRemote = &net.IPAddr{IP: []byte{127, 0, 0, 1}}

func TestNewConnectionHandler(t *testing.T) {
	type args struct {
		mockConn    func(m *MockConn)
		msgHandler  MessageHandler
		isClient    bool
		idleTimeout time.Duration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "NewConnectionHandler",
			args: args{
				mockConn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Twice()
				},
				msgHandler:  func(writer io.Writer, data []byte, dataCh <-chan []byte, close chan<- struct{}) {},
				isClient:    true,
				idleTimeout: time.Second,
			},
			wantErr: false,
		},
		{
			name: "NewConnectionHandler without idleTimeout",
			args: args{
				mockConn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Twice()
				},
				msgHandler: func(writer io.Writer, data []byte, dataCh <-chan []byte, close chan<- struct{}) {},
				isClient:   true,
			},
			wantErr: false,
		},

		{
			name: "NewConnectionHandler without connection",
			args: args{
				msgHandler:  func(writer io.Writer, data []byte, dataCh <-chan []byte, close chan<- struct{}) {},
				isClient:    true,
				idleTimeout: time.Second,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var conn net.Conn

			if tt.args.mockConn != nil {
				mockConn := NewMockConn(t)
				tt.args.mockConn(mockConn)
				conn = mockConn
			}

			got, err := NewConnectionHandler(ConnectionHandlerConfig{
				Conn:        conn,
				MsgHandler:  tt.args.msgHandler,
				IsClient:    tt.args.isClient,
				IdleTimeout: tt.args.idleTimeout,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnectionHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			assert.NotNil(t, got.conn)
			assert.NotNil(t, got.data)
			assert.NotNil(t, got.activity)
			assert.NotNil(t, got.close)
			assert.Equal(t, tt.args.isClient, got.isClient)

			assert.NotNil(t, got.reader)
			r := got.reader.(*ConnectionReader)
			assert.NotNil(t, r.reader)
			assert.NotNil(t, r.activity)
			assert.NotNil(t, r.dataCh)
			assert.Equal(t, 1024, len(r.buffer))

			assert.NotNil(t, got.msgProcessor)
			m := got.msgProcessor.(*MessageProcessor)
			assert.NotNil(t, m.writer)
			assert.NotNil(t, m.handler)
			assert.NotNil(t, m.data)
			assert.NotNil(t, m.close)
			assert.Equal(t, expectedRemote.String(), m.remote)

			assert.NotNil(t, got.keepAlive)
			ka := got.keepAlive.(*KeepAlive)
			assert.NotNil(t, ka.writer)
			assert.NotNil(t, ka.close)
			assert.NotNil(t, ka.activity)
			assert.Equal(t, expectedRemote.String(), ka.remote)
			if tt.args.idleTimeout > 0 {
				assert.Equal(t, tt.args.idleTimeout, ka.idleTimeout)
			} else {
				assert.Equal(t, IdleTimeout, ka.idleTimeout)
			}

		})
	}
}

func TestConnectionHandler_Handle(t *testing.T) {
	type fields struct {
		conn         func(m *MockConn)
		reader       func(m *MockConnReader)
		msgProcessor func(m *MockMsgProcessor)
		keepAlive    func(m *MockKeepAliveEngine)
		close        chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "Handle",
			fields: fields{
				reader: func(m *MockConnReader) {
					m.On("Read").Return().Once()
				},
				msgProcessor: func(m *MockMsgProcessor) {
					m.On("Process", mock.Anything).Return().Once()
				},
				keepAlive: func(m *MockKeepAliveEngine) {
					m.On("Run", mock.Anything).Return().Once()
				},
				close: make(chan struct{}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := NewMockConn(t)
			mockConnReader := NewMockConnReader(t)
			mockMsgProcessor := NewMockMsgProcessor(t)
			mockKeepAliveEngine := NewMockKeepAliveEngine(t)

			if tt.fields.reader != nil {
				tt.fields.reader(mockConnReader)
			}

			if tt.fields.msgProcessor != nil {
				tt.fields.msgProcessor(mockMsgProcessor)
			}

			if tt.fields.keepAlive != nil {
				tt.fields.keepAlive(mockKeepAliveEngine)
			}

			ch := &ConnectionHandler{
				conn:         mockConn,
				reader:       mockConnReader,
				msgProcessor: mockMsgProcessor,
				keepAlive:    mockKeepAliveEngine,
				close:        tt.fields.close,
			}
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				ch.Handle()
			}()
			time.AfterFunc(10*time.Millisecond, func() {
				ch.close <- struct{}{}
			})
			wg.Wait()
		})
	}
}

func TestConnectionHandler_Close(t *testing.T) {
	type fields struct {
		conn     func(m *MockConn)
		isClient bool
		data     chan []byte
		activity chan struct{}
		close    chan struct{}
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "Close Server",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Once()
					m.On("Close").Return(nil).Once()
				},
				data:     make(chan []byte),
				activity: make(chan struct{}),
				close:    make(chan struct{}),
			},
		},
		{
			name: "Close Server with connection.Close() error",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Once()
					m.On("Close").Return(errors.New("error")).Once()
				},
				data:     make(chan []byte),
				activity: make(chan struct{}),
				close:    make(chan struct{}),
			},
		},
		{
			name: "Close Client",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Once()
					m.On("Write", Stop).Return(0, nil).Once()
					m.On("Read", mock.Anything).Return(0, nil).Once()
					m.On("Read", mock.Anything).Return(0, io.EOF).Once()
					m.On("Close").Return(nil).Once()
				},
				data:     make(chan []byte),
				activity: make(chan struct{}),
				close:    make(chan struct{}),
				isClient: true,
			},
		},
		{
			name: "Close Client with connection.Write() error",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Once()
					m.On("Write", Stop).Return(0, errors.New("error")).Once()
					m.On("Close").Return(nil).Once()
				},
				data:     make(chan []byte),
				activity: make(chan struct{}),
				close:    make(chan struct{}),
				isClient: true,
			},
		},
		{
			name: "Close Client with timeout waiting for server",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("RemoteAddr").Return(expectedRemote).Once()
					m.On("Write", Stop).Return(0, nil).Once()
					m.On("Read", mock.Anything).Return(0, nil).Times(4)
					m.On("Close").Return(nil).Once()
				},
				data:     make(chan []byte),
				activity: make(chan struct{}),
				close:    make(chan struct{}),
				isClient: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := NewMockConn(t)

			if tt.fields.conn != nil {
				tt.fields.conn(mockConn)
			}

			ch := &ConnectionHandler{
				conn:     mockConn,
				isClient: tt.fields.isClient,
				data:     tt.fields.data,
				activity: tt.fields.activity,
				close:    tt.fields.close,
			}
			ch.Close()
		})
	}
}

func TestConnectionReader_Read(t *testing.T) {

	tests := []struct {
		name           string
		closeWithError error
	}{
		{
			name:           "Read with connection closed",
			closeWithError: net.ErrClosed,
		},
		{
			name:           "Read with different close error",
			closeWithError: io.EOF,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()

			buffer := make([]byte, 1024)
			dataCh := make(chan []byte)
			activity := make(chan struct{})

			cr := &ConnectionReader{
				reader:   testReader,
				buffer:   buffer,
				dataCh:   dataCh,
				activity: activity,
			}

			wgR := sync.WaitGroup{}
			wgR.Add(1)
			go func() {
				defer wgR.Done()
				<-activity
				<-activity
			}()

			wgR.Add(1)
			go func() {
				defer wgR.Done()
				data := <-dataCh
				assert.Equal(t, OpPing, data)
			}()

			wgP := sync.WaitGroup{}
			wgP.Add(1)
			go func() {
				defer wgP.Done()
				go cr.Read()
			}()

			_, _ = testWriter.Write(OpPing)
			_, _ = testWriter.Write(CRLF)
			wgR.Wait()
			_ = testWriter.CloseWithError(tt.closeWithError)
			wgP.Wait()
		})
	}
}

func TestMessageProcessor_Process(t *testing.T) {

	testWriter := &bytes.Buffer{}

	testHandler := func(writer io.Writer, data []byte, dataCh <-chan []byte, close chan<- struct{}) {
		assert.NotNil(t, writer)
		assert.NotNil(t, data)
		assert.NotNil(t, dataCh)
		assert.NotNil(t, close)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := make(chan []byte)
	closeCh := make(chan struct{})

	m := &MessageProcessor{
		writer:  testWriter,
		handler: testHandler,
		remote:  expectedRemote.String(),
		data:    data,
		close:   closeCh,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Process(ctx)
	}()

	data <- OpPing

	time.AfterFunc(10*time.Millisecond, func() {
		cancel()
	})

	wg.Wait()

}

func TestKeepAlive_Run(t *testing.T) {
	tests := []struct {
		name string
		test func(readCloser *io.PipeWriter, activity chan struct{}, cancel context.CancelFunc)
	}{
		{
			name: "KeepAlive Canceled",
			test: func(_ *io.PipeWriter, activity chan struct{}, cancel context.CancelFunc) {
				time.AfterFunc(10*time.Millisecond, func() {
					activity <- struct{}{}
				})

				time.AfterFunc(20*time.Millisecond, func() {
					cancel()
				})
			},
		},
		{
			name: "KeepAlive Writer Closed With Error",
			test: func(writeCloser *io.PipeWriter, activity chan struct{}, cancel context.CancelFunc) {
				time.AfterFunc(10*time.Millisecond, func() {
					activity <- struct{}{}
				})

				time.AfterFunc(20*time.Millisecond, func() {
					_ = writeCloser.CloseWithError(io.ErrClosedPipe)
				})
			},
		},
		{
			name: "KeepAlive Timed out",
			test: func(writeCloser *io.PipeWriter, activity chan struct{}, cancel context.CancelFunc) {
				//time.AfterFunc(10*time.Millisecond, func() {
				//	activity <- struct{}{}
				//})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(slog.LevelDebug)
			defer SetLogLevel(slog.LevelError)

			testReader, testWriter := io.Pipe()

			activity := make(chan struct{})
			closeCh := make(chan struct{})

			keepAlive := &KeepAlive{
				writer:      testWriter,
				remote:      expectedRemote.String(),
				activity:    activity,
				close:       closeCh,
				idleTimeout: 10 * time.Millisecond,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			buffer := make([]byte, 1024)
			go func() {
				for {
					n, err := testReader.Read(buffer)
					if err != nil {
						return
					}
					t.Log(buffer[:n])
					assert.True(t, bytes.Equal(Ping, buffer[:n]))
				}
			}()

			go func() {
				<-closeCh
			}()

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				keepAlive.Run(ctx)
			}()

			tt.test(testWriter, activity, cancel)

			wg.Wait()
		})
	}
}
