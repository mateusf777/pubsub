# PUBSUB (lite/naive implementation)

The goal of this project is to learn how to implement a basic server and protocol.
It's just another project for learning and recreational programming.

## THERE'S NO SECURITY!!!!

## Disclaimer
The protocol here is not my idea. Although, I have not copied the code, I have used the same operations from [nats protocol](https://docs.nats.io/nats-protocol/nats-protocol#protocol-messages).
I highly recommend [nats](https://nats.io) if you need a connective technology.

### Build
```
go build -o ps-server         // build the pubsub server
go build ./example/subscriber // build the subscriber example
go build ./example/publisher  // build the publisher example
go build ./example/request    // build the request example
go build ./example/queue      // build the queue example
```

### Run

`./ps-server`

If you want to run the examples, first start subscriber in a different terminal, then the order does not matter.
Attention!! The publisher example triggers 1000000 messages 8x concurrently as fast as it can, it will get "a bit" of cpu.
You can adjust it in the `./example/common.go` and build it again: 
```
const (
	Routines = 8
	Messages = 1000000
)
```

### Simple subscribe/publish - test with telnet
In a terminal type
```
telnet localhost 9999
``` 
Then, type
```
sub test 1                                                                      
```
In another terminal type
```
telnet localhost 9999
```
Then, type
```
pub test
test
```
The first terminal should receive
```
MSG test 1                                                                      
test   
```
To quit in both terminals, type
```
stop
```
