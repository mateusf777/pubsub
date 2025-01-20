package core

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

func TestNewConnectionReader(t *testing.T) {
	expectedConn := NewMockConn(t)
	expectedBufferSize := 10
	expectedData := make(chan []byte)

	type args struct {
		conn       Conn
		bufferSize int
		DataChan   chan []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *ConnectionReader
		wantErr bool
	}{
		{
			name: "Create",
			args: args{
				conn:       expectedConn,
				bufferSize: expectedBufferSize,
				DataChan:   expectedData,
			},
			want: &ConnectionReader{
				conn:   expectedConn,
				buffer: make([]byte, expectedBufferSize),
				dataCh: expectedData,
			},
			wantErr: false,
		},
		{
			name: "Create with default buffer size",
			args: args{
				conn:     expectedConn,
				DataChan: expectedData,
			},
			want: &ConnectionReader{
				conn:   expectedConn,
				buffer: make([]byte, 1024),
				dataCh: expectedData,
			},
			wantErr: false,
		},
		{
			name: "Try to create without Connection",
			args: args{
				bufferSize: 10,
				DataChan:   expectedData,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Try to create without data channel",
			args: args{
				conn:       expectedConn,
				bufferSize: 10,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConnectionReader(ConnectionReaderConfig{
				Conn:       tt.args.conn,
				BufferSize: tt.args.bufferSize,
				DataChan:   tt.args.DataChan,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnectionReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConnectionReader() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionReader_Read(t *testing.T) {
	mockedBuffer := make([]byte, 1024)
	mockedDataCh := make(chan []byte)

	type fields struct {
		conn   func(m *MockConn)
		buffer []byte
		dataCh chan []byte
	}
	tests := []struct {
		name           string
		fields         fields
		expectedResult []byte
	}{
		{
			name: "Read",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("Read", mock.MatchedBy(func(buf []byte) bool {
						if !bytes.Equal(buf, mockedBuffer) {
							return false
						}
						buf[0], buf[1], buf[2], buf[3], buf[4], buf[5] = 'P', 'I', 'N', 'G', '\r', '\n'
						return true
					})).Return(6, nil).Once()
					m.On("Read", mock.Anything).Return(0, errors.New(ClosedErr)).Once()
				},
				buffer: mockedBuffer,
				dataCh: mockedDataCh,
			},
			expectedResult: OpPing,
		},
		{
			name: "Multiple reads",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("Read", mock.MatchedBy(func(buf []byte) bool {
						if !bytes.Equal(buf, mockedBuffer) {
							return false
						}
						buf[0], buf[1], buf[2], buf[3] = 'P', 'I', 'N', 'G'
						return true
					})).Return(4, nil).Once()
					m.On("Read", mock.MatchedBy(func(buf []byte) bool {
						if !bytes.Equal(buf, mockedBuffer) {
							return false
						}
						buf[0], buf[1] = '\r', '\n'
						return true
					})).Return(4, nil).Once()
					m.On("Read", mock.Anything).Return(0, errors.New(ClosedErr)).Once()
				},
				buffer: mockedBuffer,
				dataCh: mockedDataCh,
			},
			expectedResult: OpPing,
		},
		{
			name: "Read error",
			fields: fields{
				conn: func(m *MockConn) {
					m.On("Read", mock.Anything).Return(0, errors.New("error")).Once()
				},
				buffer: mockedBuffer,
				dataCh: mockedDataCh,
			},
			expectedResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := NewMockConn(t)
			if tt.fields.conn != nil {
				tt.fields.conn(mockConn)
			}

			cr := &ConnectionReader{
				conn:   mockConn,
				buffer: tt.fields.buffer,
				dataCh: tt.fields.dataCh,
			}

			cg := sync.WaitGroup{}
			var result []byte
			cg.Add(1)
			go func() {
				defer cg.Done()
				cxt, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				defer cancel()

				select {
				case <-cxt.Done():
					return
				case result = <-cr.dataCh:
					return
				}
			}()

			go cr.Read()
			cg.Wait()

			if !bytes.Equal(result, tt.expectedResult) {
				t.Errorf("Read() got = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestNewKeepAlive(t *testing.T) {

	expectedConn := NewMockConn(t)
	expectedResetCh := make(chan bool)
	expectedStopCh := make(chan bool)
	expectedCloseCh := make(chan bool)
	expectedIdleTimeout := time.Second

	type args struct {
		cfg KeepAliveConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *KeepAlive
		wantErr bool
	}{
		{
			name: "Create",
			args: args{
				cfg: KeepAliveConfig{
					Conn:        expectedConn,
					ResetCh:     expectedResetCh,
					StopCh:      expectedStopCh,
					CloseCh:     expectedCloseCh,
					IdleTimeout: expectedIdleTimeout,
				},
			},
			want: &KeepAlive{
				conn:        expectedConn,
				reset:       expectedResetCh,
				stop:        expectedStopCh,
				close:       expectedCloseCh,
				idleTimeout: expectedIdleTimeout,
			},
			wantErr: false,
		},
		{
			name: "Create with default timeout",
			args: args{
				cfg: KeepAliveConfig{
					Conn:    expectedConn,
					ResetCh: expectedResetCh,
					StopCh:  expectedStopCh,
					CloseCh: expectedCloseCh,
				},
			},
			want: &KeepAlive{
				conn:        expectedConn,
				reset:       expectedResetCh,
				stop:        expectedStopCh,
				close:       expectedCloseCh,
				idleTimeout: IdleTimeout,
			},
			wantErr: false,
		},
		{
			name: "Try to create without connection",
			args: args{
				cfg: KeepAliveConfig{
					ResetCh:     expectedResetCh,
					StopCh:      expectedStopCh,
					CloseCh:     expectedCloseCh,
					IdleTimeout: expectedIdleTimeout,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Try to create without reset channel",
			args: args{
				cfg: KeepAliveConfig{
					Conn:        expectedConn,
					StopCh:      expectedStopCh,
					CloseCh:     expectedCloseCh,
					IdleTimeout: expectedIdleTimeout,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Try to create without stop channel",
			args: args{
				cfg: KeepAliveConfig{
					Conn:        expectedConn,
					ResetCh:     expectedResetCh,
					CloseCh:     expectedCloseCh,
					IdleTimeout: expectedIdleTimeout,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Try to create without close channel",
			args: args{
				cfg: KeepAliveConfig{
					Conn:        expectedConn,
					ResetCh:     expectedResetCh,
					StopCh:      expectedStopCh,
					IdleTimeout: expectedIdleTimeout,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKeepAlive(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeepAlive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewKeepAlive() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeepAlive_Run(t *testing.T) {

	resetCh := make(chan bool)
	stopCh := make(chan bool)
	closeCh := make(chan bool)
	idleTimeout := 100 * time.Millisecond

	mockedAddr := &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 1234,
	}

	type fields struct {
		reset       chan bool
		stop        chan bool
		close       chan bool
		idleTimeout time.Duration
	}
	tests := []struct {
		name     string
		fields   fields
		mockConn func(m *MockConn)
		mockOps  func(resetCh, stopCh, closeCh chan bool)
	}{
		{
			name: "Reset and stop",
			fields: fields{
				reset:       resetCh,
				stop:        stopCh,
				close:       closeCh,
				idleTimeout: idleTimeout,
			},
			mockConn: func(m *MockConn) {
				m.On("RemoteAddr").Return(mockedAddr).Twice()
			},
			mockOps: func(resetCh, stopCh, closeCh chan bool) {
				resetCh <- true
				stopCh <- true
			},
		},
		{
			name: "Check and stop",
			fields: fields{
				reset:       resetCh,
				stop:        stopCh,
				close:       closeCh,
				idleTimeout: 10 * time.Millisecond,
			},
			mockConn: func(m *MockConn) {
				m.On("RemoteAddr").Return(mockedAddr).Times(3)
				m.On("Write", mock.Anything).Return(0, nil)
			},
			mockOps: func(resetCh, stopCh, closeCh chan bool) {
				time.Sleep(15 * time.Millisecond)
				stopCh <- true
			},
		},
		{
			name: "Check twice and close",
			fields: fields{
				reset:       resetCh,
				stop:        stopCh,
				close:       closeCh,
				idleTimeout: time.Millisecond,
			},
			mockConn: func(m *MockConn) {
				m.On("RemoteAddr").Return(mockedAddr).Times(5)
				m.On("Write", mock.Anything).Return(0, nil).Twice()
			},
			mockOps: func(resetCh, stopCh, closeCh chan bool) {
				<-closeCh
			},
		},
		{
			name: "Check, write broken pipe, close",
			fields: fields{
				reset:       resetCh,
				stop:        stopCh,
				close:       closeCh,
				idleTimeout: idleTimeout,
			},
			mockConn: func(m *MockConn) {
				m.On("RemoteAddr").Return(mockedAddr).Twice()
				m.On("Write", mock.Anything).Return(0, errors.New("broken pipe"))
			},
			mockOps: func(resetCh, stopCh, closeCh chan bool) {
				<-closeCh
			},
		},
	}
	for _, tt := range tests {
		mockedConn := NewMockConn(t)

		if tt.mockConn != nil {
			tt.mockConn(mockedConn)
		}

		t.Run(tt.name, func(t *testing.T) {
			k := &KeepAlive{
				conn:        mockedConn,
				reset:       tt.fields.reset,
				stop:        tt.fields.stop,
				close:       tt.fields.close,
				idleTimeout: tt.fields.idleTimeout,
			}

			SetLogLevel(slog.LevelDebug)
			defer SetLogLevel(slog.LevelError)

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				k.Run()
			}()

			tt.mockOps(resetCh, stopCh, closeCh)

			wg.Wait()
		})
	}
}
