# PUBSUB [![Go](https://github.com/mateusf777/pubsub/actions/workflows/go.yml/badge.svg)](https://github.com/mateusf777/pubsub/actions/workflows/go.yml)

## Overview
This project is a **learning-oriented** implementation of a basic pub/sub server and protocol. It was developed to gain a deeper understanding of how pub/sub systems work at a fundamental level.

### **Disclaimer**
This project was inspired by the [NATS protocol](https://docs.nats.io/nats-protocol/nats-protocol#protocol-messages). While it follows similar operations, it is not a replacement for NATS, nor does it aim to be production-ready. If you need a robust pub/sub system for real-world use, consider using [NATS](https://nats.io).

### **Security Warning**
🚨 **This implementation is under development. Authentication and TLS security features are ongoing.** Use it for learning and experimentation only.

---


## PUBSUB Protocol

### Overview
This document describes the protocol commands used in the PUBSUB server. The protocol consists of **client-to-server** and **server-to-client** messages for handling publishing, subscribing, connection management, and health checks.

---

### Client-to-Server Commands

#### Publish Message (PUB)
**Syntax:**
```
PUB <subject> [reply_id] \r\n
[msg] \r\n
```
**Description:**
- Publishes a message to a specified subject.
- Optionally includes a `reply_id` for request-response patterns.

**Example:**
```
PUB news.update
Hello, world!
```

---

#### Subscribe to Subject (SUB)
**Syntax:**
```
SUB <subject> <sub_id> [group] \r\n
```
**Description:**
- Subscribes to a subject.
- `sub_id` is the identifier for the subscription.
- `group` (optional) is used for queue groups.

**Example:**
```
SUB news.update 1
```

---

#### Unsubscribe from Subject (UNSUB)
**Syntax:**
```
UNSUB <sub_id> \r\n
```
**Description:**
- Unsubscribes from a subject using the given `sub_id`.

**Example:**
```
UNSUB 1
```

---

#### Disconnect (STOP)
**Syntax:**
```
STOP \r\n
```
**Description:**
- Informs the server that the client is closing the connection.

**Example:**
```
STOP
```

---

#### Respond to Server Ping (PONG)
**Syntax:**
```
PONG \r\n
```
**Description:**
- Sent in response to a `PING` message from the server.

**Example:**
```
PONG
```

---

### Server-to-Client Commands

#### Health Check (PING)
**Syntax:**
```
PING \r\n
```
**Description:**
- Sent by the server to check if the client is alive.
- Clients must respond with `PONG`.

**Example:**
```
PING
```

---

#### Deliver a Message (MSG)
**Syntax:**
```
MSG <subject> <sub_id> [reply-to] \r\n
[payload] \r\n
```
**Description:**
- Sent by the server to deliver a message to a subscriber.
- Includes the subject, `sub_id`, and optional `reply-to`.

**Example:**
```
MSG news.update 1
Breaking news: Something happened!
```

---

#### Acknowledge Command (+OK)
**Syntax:**
```
+OK \r\n
```
**Description:**
- Sent by the server to acknowledge receipt of a valid command.

**Example:**
```
+OK
```

---

#### Error Response (-ERR)
**Syntax:**
```
-ERR <error> \r\n
```
**Description:**
- Sent by the server when a protocol error occurs.

**Example:**
```
-ERR Invalid command
```

---

### **Notes**
- All commands must be terminated with (carriage return + line feed), ensuring proper message parsing by the server.
- The PUBSUB server processes these commands in a **stateless, event-driven manner**.
- Clients must handle `PING` messages by responding with `PONG` to maintain the connection.

---


## **Build Instructions**

Run the following commands to build the server and example applications:

```
cd server
go build -o ../build/ps-server ./cmd/pubsub

cd ../example
go build -o ../build/queue ./queue
go build -o ../build/subscriber ./subscriber
go build -o ../build/publisher ./publisher
go build -o ../build/request ./request

cd ../build
```

---

## **Running the Server**
To start the pub/sub server:

```
./ps-server
```

---

## **Usage Examples**

### **1. Subscribe and Publish Example**
#### Start a Subscriber
```
./subscriber
```

#### Send a Request from Another Terminal
```
./request
```
##### Example Output
```
{"time":"<timestamp>","level":"INFO","msg":"request time"}
{"time":"<timestamp>","level":"INFO","msg":"now","data":"<formatted time>"}
```

#### Launch a Queue Subscriber
```
./queue
```
##### Example Output
```
{"time":"<timestamp>","level":"INFO","msg":"Launching subscribers","queue":3}
{"time":"<timestamp>","level":"INFO","msg":"Queue, successfully subscribed","queue":0}
{"time":"<timestamp>","level":"INFO","msg":"Queue, successfully subscribed","queue":1}
{"time":"<timestamp>","level":"INFO","msg":"Queue, successfully subscribed","queue":2}
...
{"time":"<timestamp>","level":"INFO","msg":"Received all messages"}
```

#### Publish Messages
```
./publisher
```
##### Example Output
```
{"time":"<timestamp>","level":"INFO","msg":"Sending messages","count":10000}
{"time":"<timestamp>","level":"INFO","msg":"Done"}
{"time":"<timestamp>","level":"INFO","msg":"Connection closed"}
```

##### Example Output in Subscriber Terminal
```
{"time":"<timestamp>","level":"INFO","msg":"received","count":10000}
```

---

## **2. Simple Subscribe/Publish via Telnet**
You can interact with the pub/sub server using `telnet`.

### **Start the Server**
```
./ps-server
```

### **Subscribe to a Topic**
```
telnet localhost 9999
```
Then type:
```
sub test 1
```

### **Publish a Message**
Open another terminal and connect via telnet:
```
telnet localhost 9999
```
Then type:
```
pub test
test
```

### **Example Output (Subscriber Terminal)**
```
MSG test 1
test
```

### **Stopping the Server**
To quit in both terminals, type:
```
stop
```

---

## **Final Notes**
This project is a **learning exercise** and not meant for production.
