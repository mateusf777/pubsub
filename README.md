# PUBSUB [![Go](https://github.com/mateusf777/pubsub/actions/workflows/go.yml/badge.svg)](https://github.com/mateusf777/pubsub/actions/workflows/go.yml)

## Overview

This project is a **learning-oriented** implementation of a basic pub/sub server and protocol. It was developed to gain a deeper understanding of how pub/sub systems work at a fundamental level.

### Disclaimer

This project was inspired by the [NATS protocol](https://docs.nats.io/nats-protocol/nats-protocol#protocol-messages). While it follows similar operations, it is not a replacement for NATS, nor does it aim to be production-ready. If you need a robust pub/sub system for real-world use, consider using [NATS](https://nats.io).

### Warning

🚨 **This implementation is under development.** Use it for learning and experimentation only.

---

## Features

* ✅ Lightweight pub/sub protocol (inspired by NATS)
* ✅ Custom TCP server and client
* ✅ PUB, SUB, UNSUB, STOP, PING/PONG message handling
* ✅ Queue groups for load-balanced subscriptions
* ✅ TLS support for encrypted communication
* ✅ Authentication system
* ✅ Multi-tenancy when the server is configured with a CA
* ⚙️ Metrics (planned)

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

#### Subscribe to Subject (SUB)

```
SUB <subject> <sub_id> [group] \r\n
```

#### Unsubscribe from Subject (UNSUB)

```
UNSUB <sub_id> \r\n
```

#### Disconnect (STOP)

```
STOP \r\n
```

#### Respond to Server Ping (PONG)

```
PONG \r\n
```

---

### Server-to-Client Commands

#### Health Check (PING)

```
PING \r\n
```

#### Deliver a Message (MSG)

```
MSG <subject> <sub_id> [reply-to] \r\n
[payload] \r\n
```

#### Acknowledge Command (+OK)

```
+OK \r\n
```

#### Error Response (-ERR)

```
-ERR <error> \r\n
```

---

### Notes

* All commands must be terminated with (carriage return + line feed), ensuring proper message parsing by the server.
* The PUBSUB server processes these commands in a stateless, event-driven manner.
* Clients must handle `PING` messages by responding with `PONG` to maintain the connection.

---

## Build Instructions

Run the following command to build the server and example applications:

```bash
./build.sh
```

---

## Running the Server

To start the pub/sub server:

```bash
./build/ps-server
```

---

### 🔐 TLS Support

TLS is supported using environment variables for configuration.

#### ➔ Basic TLS (Server Authentication Only)

To start the server with TLS enabled (clients verify the server certificate):

```bash
PUBSUB_TLS_CERT=./certs/server.crt \
PUBSUB_TLS_KEY=./certs/server.key \
PUBSUB_ADDRESS=0.0.0.0:9443 \
./build/ps-server
```

This enables encrypted client-server communication over a secure TLS connection. The server presents a certificate to the client, but does **not** verify client certificates.

---

#### ➔ Mutual TLS (Client Authentication with CA)

To enable client certificate verification, you must also provide a certificate authority (CA) used to sign client certificates:

```bash
PUBSUB_TLS_CERT=./certs/server.crt \
PUBSUB_TLS_KEY=./certs/server.key \
PUBSUB_TLS_CA=./certs/ca.crt \
PUBSUB_ADDRESS=0.0.0.0:9443 \
./build/ps-server
```

Environment variable summary:

* `PUBSUB_TLS_CERT`: Path to the server certificate.
* `PUBSUB_TLS_KEY`: Path to the server private key.
* `PUBSUB_TLS_CA`: **Used to verify client certificates.** Only clients with certificates signed by this CA will be accepted.
* `PUBSUB_ADDRESS`: Address the server should bind to.

When using `PUBSUB_TLS_CA`, the server will **require** and **verify** client certificates during the TLS handshake. Connections without valid certificates will be rejected. (See `integration_tls_ca_test.go` for an example of this in practice.)

**Note:**

* If the server is not configured with TLS, all connections are accepted (insecure).
* If TLS is configured but `PUBSUB_TLS_CA` is **not** set, the server provides secure transport only—no client identity verification or tenant isolation.
* When `PUBSUB_TLS_CA` is provided, **tenant isolation is enforced**: messages are only routed between connections with same certificate.

---

## Usage Examples

### 1. Subscribe and Publish Example

Start a subscriber:

```bash
./build/subscriber
```

Send a request from another terminal:

```bash
./build/request
```

**Example Output:**

```
{"time":"<timestamp>","level":"INFO","msg":"request time"}
{"time":"<timestamp>","level":"INFO","msg":"now","data":"<formatted time>"}
```

Launch a queue subscriber:

```bash
./build/queue
```

**Example Output:**

```
{"time":"<timestamp>","level":"INFO","msg":"Launching subscribers","queue":3}
...
{"time":"<timestamp>","level":"INFO","msg":"Received all messages"}
```

Publish messages:

```bash
./publisher
```

**Example Output:**

```
{"time":"<timestamp>","level":"INFO","msg":"Sending messages","count":10000}
{"time":"<timestamp>","level":"INFO","msg":"Done"}
{"time":"<timestamp>","level":"INFO","msg":"Connection closed"}
```

**Subscriber Output:**

```
{"time":"<timestamp>","level":"INFO","msg":"received","count":10000}
```

---

## 2. Simple Subscribe/Publish via Telnet

Start the server:

```bash
./build/ps-server
```

Subscribe to a topic:

```bash
telnet localhost 9999
```

Then type:

```
SUB test 1
```

Publish a message from another terminal:

```bash
telnet localhost 9999
```

Then type:

```
PUB test
Hello
```

**Expected Output on Subscriber Terminal:**

```
MSG test 1
Hello
```

To disconnect:

```
STOP
```

---

## Project Goals

This project was built to learn and demonstrate:

* Low-level TCP networking with Go
* Designing a line-based protocol from scratch
* Building concurrent systems with minimal external dependencies
* Writing testable and structured code for infrastructure services

---

## Final Notes

This project is a **learning exercise** and not meant for production.
