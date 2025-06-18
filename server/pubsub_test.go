package server

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
				return
			}
			if got.msgCh == nil {
				t.Errorf("NewPubSub() channel not initialized")
				return
			}
			if got.handlersMap == nil {
				t.Errorf("NewPubSub() handlers not initialized")
				return
			}

			timer := time.NewTimer(time.Second)
			for {
				select {
				case <-timer.C:
					t.Errorf("the pubsub was not started and timed out")
					return
				default:
					if got.running {
						return
					}
				}
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
			name: "Subscribe without remote",
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

func TestPubSub_Unsubscribe(t *testing.T) {
	handlers := new(sync.Map)

	type fields struct {
		msgCh       chan Message
		handlersMap *sync.Map
	}
	type args struct {
		subject string
		client  string
		id      int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		preTest func()
		wantErr assert.ErrorAssertionFunc
		verify  bool
	}{
		{
			name: "Unsubscribe",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				subject: "test",
				client:  "test",
				id:      1,
			},
			preTest: func() {
				handlers.Store("test", []HandlerSubject{{
					subject: "test",
					client:  "test",
					id:      1,
				}})
			},
			wantErr: assert.NoError,
			verify:  true,
		},
		{
			name: "Unsubscribe without subject",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				client: "test",
				id:     1,
			},
			wantErr: assert.Error,
		},
		{
			name: "Unsubscribe without remote",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				subject: "test",
				id:      1,
			},
			wantErr: assert.Error,
		},
		{
			name: "Unsubscribe but the subject is not stored",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				subject: "test",
				client:  "test",
				id:      1,
			},
			preTest: func() {
				handlers.Delete("test")
			},
			wantErr: assert.Error,
		},
		{
			name: "Unsubscribe but the handler is not stored",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				subject: "test",
				client:  "test",
				id:      1,
			},
			preTest: func() {
				handlers.Store("test", []HandlerSubject{})
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PubSub{
				msgCh:       tt.fields.msgCh,
				handlersMap: tt.fields.handlersMap,
			}

			if tt.preTest != nil {
				tt.preTest()
			}

			tt.wantErr(t, ps.Unsubscribe(tt.args.subject, tt.args.client, tt.args.id), fmt.Sprintf("Unsubscribe(%v, %v, %v)", tt.args.subject, tt.args.client, tt.args.id))

			if tt.verify {
				v, _ := tt.fields.handlersMap.Load(tt.args.subject)
				hs, ok := v.([]HandlerSubject)
				assert.True(t, ok)
				assert.Equal(t, len(hs), 0)
			}
		})
	}
}

func TestPubSub_UnsubAll(t *testing.T) {
	handlers := new(sync.Map)

	type fields struct {
		msgCh       chan Message
		handlersMap *sync.Map
	}
	type args struct {
		client string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		preTest func()
		verify  bool
	}{
		{
			name: "UnsubAll",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: handlers,
			},
			args: args{
				client: "test",
			},
			preTest: func() {
				handlers.Store("test", []HandlerSubject{
					{subject: "test", client: "test", id: 1, handler: func(msg Message) {}},
					{subject: "test", client: "test", id: 2, handler: func(msg Message) {}},
				})
			},
			verify: true,
		},
		{
			name: "UnsubAll but remote has no handlers stored",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			args: args{
				client: "test",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PubSub{
				msgCh:       tt.fields.msgCh,
				handlersMap: tt.fields.handlersMap,
			}
			if tt.preTest != nil {
				tt.preTest()
			}
			ps.UnsubAll(tt.args.client)

			if tt.verify {
				v, ok := tt.fields.handlersMap.Load(tt.args.client)
				assert.True(t, ok)
				hs, ok := v.([]HandlerSubject)
				assert.True(t, ok)
				assert.Equal(t, len(hs), 0)
			}
		})
	}
}

func TestPubSub_run(t *testing.T) {
	expectedHandlers := []HandlerSubject{
		{subject: "test", client: "test", id: 1},
	}
	expectedMessage := Message{
		Subject: "test",
		Data:    []byte("test"),
	}

	type fields struct {
		msgCh       chan Message
		handlersMap *sync.Map
		mockRouter  func(m *MockRouter)
	}
	tests := []struct {
		name    string
		fields  fields
		preTest func(m *sync.Map)
		message *Message
	}{
		{
			name: "Run",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
				mockRouter: func(m *MockRouter) {
					m.On("Route", expectedHandlers, expectedMessage).Return(nil)
				},
			},
			preTest: func(m *sync.Map) {
				m.Store("test", expectedHandlers)
			},
			message: &expectedMessage,
		},
		{
			name: "Run, stop, no routing is done",
			fields: fields{
				msgCh:       make(chan Message),
				handlersMap: new(sync.Map),
			},
			preTest: func(m *sync.Map) {
				m.Store("test", expectedHandlers)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRouter := NewMockRouter(t)

			if tt.preTest != nil {
				tt.preTest(tt.fields.handlersMap)
			}

			if tt.fields.mockRouter != nil {
				tt.fields.mockRouter(mockRouter)
			}

			ps := &PubSub{
				msgCh:       tt.fields.msgCh,
				handlersMap: tt.fields.handlersMap,
				router:      mockRouter,
			}

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				ps.run()
			}()

			if tt.message != nil {
				tt.fields.msgCh <- *tt.message
			}

			close(tt.fields.msgCh)
			wg.Wait()

		})
	}
}

func Test_msgRouter_Route(t *testing.T) {
	type args struct {
		subHandlers []HandlerSubject
		msg         Message
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Route",
			args: args{
				subHandlers: []HandlerSubject{
					{subject: "test", client: "test", id: 1, handler: func(msg Message) {
						assert.Equal(t, "test", msg.Subject)
						assert.Equal(t, []byte("test"), msg.Data)
					}},
				},
				msg: Message{
					Subject: "test",
					Data:    []byte("test"),
				},
			},
		},
		{
			name: "Route for groups",
			args: args{
				subHandlers: []HandlerSubject{
					{subject: "test", client: "test", id: 1, group: "test", handler: func(msg Message) {
						assert.Equal(t, "test", msg.Subject)
						assert.Equal(t, []byte("test"), msg.Data)
					}},
				},
				msg: Message{
					Subject: "test",
					Data:    []byte("test"),
				},
			},
		},
		{
			name: "Route but there's no handler",
			args: args{
				subHandlers: []HandlerSubject{},
				msg: Message{
					Subject: "test",
					Data:    []byte("test"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &msgRouter{}
			r.Route(tt.args.subHandlers, tt.args.msg)
		})
	}
}
