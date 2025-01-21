package server

import (
	"bytes"
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscribe(t *testing.T) {
	ps := NewPubSub(PubSubConfig{})

	err := ps.Subscribe("test", "client", func(msg Message) {
		t.Logf("%v", msg)
	})
	if err != nil {
		t.Errorf("should not return an error, %+v", err)
	}

	if _, ok := ps.handlersMap.Load("test"); !ok {
		t.Errorf("the subscriber was not correctly persisted")
	}

	if !ps.hasSubscriber("test") {
		t.Errorf("the subscriber was not correctly persisted")
	}
}

func TestReceiveMessage(t *testing.T) {
	ps := NewPubSub(PubSubConfig{})
	test := []byte("test")
	received := false
	err := ps.Subscribe("test", "client", func(msg Message) {
		received = true
		if msg.Subject != "test" || bytes.Compare(msg.Data, test) != 0 {
			t.Errorf("the message was not what the expected, %v", msg)
		}
		t.Logf("%v", msg)
	})
	if err != nil {
		t.Errorf("should not return an error, %+v", err)
	}

	ps.Publish("test", test)

	timer := time.NewTimer(time.Second)
	for {
		select {
		case <-timer.C:
			t.Errorf("the message was not received and timed out")
		default:
			if received {
				break
			}
		}
		break
	}
}

func TestNewPubSub(t *testing.T) {
	type args struct {
		cfg PubSubConfig
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Create with default config",
			args: args{},
		},
		{
			name: "Create with config",
			args: args{
				cfg: PubSubConfig{
					MsgCh:       make(chan Message),
					HandlersMap: new(sync.Map),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPubSub(tt.args.cfg)
			if got == nil {
				t.Errorf("NewPubSub() returned nil")
			}
			if got.msgCh == nil {
				t.Errorf("NewPubSub() channel not initialized")
			}
			if got.handlersMap == nil {
				t.Errorf("NewPubSub() handlers not initialized")
			}
			if got.running {
				t.Errorf("PubSub already running")
			}

			timer := time.NewTimer(time.Second)
			for {
				select {
				case <-timer.C:
					t.Errorf("the pubsub was not started and timed out")
				default:
					if got.running {
						break
					}
				}
				break
			}
		})
	}
}

func TestPubSub_Stop(t *testing.T) {

	ps := NewPubSub(PubSubConfig{
		MsgCh: make(chan Message),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ps.msgCh:
			break
		case <-ctx.Done():
			t.Errorf("Took too long to stop, somthing is wrong")
			break
		}
	}()

	ps.Stop()
	wg.Wait()

}

func TestPubSub_Publish(t *testing.T) {
	subject := "test"
	handlers := new(sync.Map)
	handlers.Store(subject, []HandlerSubject{{}})

	type fields struct {
		msgCh       chan Message
		handlersMap *sync.Map
	}
	type args struct {
		subject string
		data    []byte
		opts    []PubOpt
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantMsg     Message
		wantDropMsg bool
	}{
		{
			name: "Publish",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				subject: subject,
				data:    []byte("test"),
			},
			wantMsg: Message{
				Subject: subject,
				Data:    []byte("test"),
			},
		},
		{
			name: "Publish with reply",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				subject: subject,
				data:    []byte("test"),
				opts:    []PubOpt{WithReply("reply")},
			},
			wantMsg: Message{
				Subject: subject,
				Data:    []byte("test"),
				Reply:   "reply",
			},
		},
		{
			name: "Publish, but there are no handlers",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				subject: subject,
				data:    []byte("test"),
			},
			wantDropMsg: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PubSub{
				msgCh:       tt.fields.msgCh,
				handlersMap: tt.fields.handlersMap,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()

				select {
				case msg := <-ps.msgCh:
					if !reflect.DeepEqual(msg, tt.wantMsg) {
						t.Errorf("the message was not what the expected, %v", msg)
					}
					break
				case <-ctx.Done():
					if tt.wantDropMsg {
						break
					}
					t.Errorf("Took too long to receive message, somthing is wrong")
					break
				}
			}()

			ps.Publish(tt.args.subject, tt.args.data, tt.args.opts...)
			wg.Wait()
		})
	}
}

func TestPubSub_Subscribe(t *testing.T) {
	type fields struct {
		msgCh       chan Message
		handlersMap *sync.Map
	}
	type args struct {
		subject string
		client  string
		handler Handler
		opts    []SubOpt
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantErr     bool
		wantHandler HandlerSubject
	}{
		{
			name: "Subscribe",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				subject: "test",
				client:  "test",
				handler: func(msg Message) {},
			},
			wantErr: false,
			wantHandler: HandlerSubject{
				subject: "test",
				client:  "test",
			},
		},
		{
			name: "Subscribe with group",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				subject: "test",
				client:  "test",
				handler: func(msg Message) {},
				opts:    []SubOpt{WithGroup("test")},
			},
			wantErr: false,
			wantHandler: HandlerSubject{
				subject: "test",
				client:  "test",
				group:   "test",
			},
		},
		{
			name: "Subscribe with id",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				subject: "test",
				client:  "test",
				handler: func(msg Message) {},
				opts:    []SubOpt{WithID(1)},
			},
			wantErr: false,
			wantHandler: HandlerSubject{
				subject: "test",
				client:  "test",
				id:      1,
			},
		},
		{
			name: "Subscribe without subject",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				client:  "test",
				handler: func(msg Message) {},
			},
			wantErr: true,
		},
		{
			name: "Subscribe without handler",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				subject: "test",
				client:  "test",
			},
			wantErr: true,
		},
		{
			name: "Subscribe without client",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				subject: "test",
				handler: func(msg Message) {},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PubSub{
				msgCh:       tt.fields.msgCh,
				handlersMap: tt.fields.handlersMap,
			}

			if err := ps.Subscribe(tt.args.subject, tt.args.client, tt.args.handler, tt.args.opts...); (err != nil) != tt.wantErr {
				t.Errorf("Subscribe() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				v, ok := tt.fields.handlersMap.Load("test")
				assert.True(t, ok)
				hs, ok := v.([]HandlerSubject)
				assert.True(t, ok)
				assert.Equal(t, len(hs), 1)
				assert.NotNil(t, hs[0].handler)
				hs[0].handler = nil
				assert.Equal(t, hs[0], tt.wantHandler)
			}

		})
	}
}
