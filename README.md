# PUBSUB (lite/naive implementation)

The goal of this project is to learn how to implement a basic server and protocol.
It's just another project for learning and recreational programming.

## THERE'S NO SECURITY!!!!

## Disclaimer
The protocol here is not my idea. Although, I have not copied the code, I have used the same operations from [nats protocol](https://docs.nats.io/nats-protocol/nats-protocol#protocol-messages).
I highly recommend [nats](https://nats.io) if you need a connective technology.

### Build
`go build -o ps-server`

### Run

`./ps-server`

### Simple subscribe/publish (for now with telnet)
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
