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

## Health Checks

The server provides HTTP health check endpoints (enabled by default):

**Health Endpoint** (`/health` or `/healthz`):
- Always returns 200 OK if the process is running
- Useful for liveness probes in Kubernetes
- Returns JSON with status, uptime, and TLS info

**Readiness Endpoint** (`/ready` or `/readyz`):
- Returns 200 OK when server is ready to accept connections
- Returns 503 during startup
- Useful for readiness probes in Kubernetes

**Configuration**:
```bash
# Health check address (default: 0.0.0.0:8080)
PUBSUB_HEALTH_ADDRESS=0.0.0.0:8080

# Enable/disable health checks (default: true)
PUBSUB_ENABLE_HEALTH=true
```

**Example**:
```bash
# Start server with health checks
./build/ps-server

# Check health
curl http://localhost:8080/health
# Response: {"status":"healthy","timestamp":"...","uptime":"1m30s","tls":false}

# Check readiness
curl http://localhost:8080/ready
# Response: {"ready":true,"timestamp":"...","message":"Server is ready"}
```

---

## Development

### Using the Makefile

The project includes a Makefile for common development tasks:

```bash
# Build all binaries
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run linters
make lint

# Run security checks
make govulncheck

# Build Docker image
make docker-build

# Clean build artifacts
make clean

# Install development tools
make install-tools

# See all available targets
make help
```

### Running Tests

Tests require mockery for mock generation:
```bash
make install-tools
make test
```

### CI/CD

The project uses GitHub Actions with:
- **Lint job**: Format checking, go vet, staticcheck, golangci-lint
- **Security job**: govulncheck, Trivy filesystem and container scanning
- **Build job**: Compilation, unit tests, integration tests
- Security scan results uploaded to GitHub Security tab

---

## Deployment

### Docker

Build and run with Docker:
```bash
docker build -t pubsub:latest -f server/Dockerfile .
docker run -p 9999:9999 -p 8080:8080 pubsub:latest
```

With TLS:
```bash
docker run \
  -e PUBSUB_TLS_CERT=/certs/server.crt \
  -e PUBSUB_TLS_KEY=/certs/server.key \
  -e PUBSUB_ADDRESS=0.0.0.0:9443 \
  -v $(pwd)/certs:/certs \
  -p 9443:9443 -p 8080:8080 \
  pubsub:latest
```

### Kubernetes

See [k8s/README.md](k8s/README.md) for detailed Kubernetes deployment instructions.

Quick deploy:
```bash
kubectl apply -f k8s/deployment.yaml
```

Features:
- 3 replicas with auto-scaling (HPA)
- Health and readiness probes
- Security best practices (non-root, read-only filesystem, dropped capabilities)
- Resource limits and requests

---

## Security

This project implements several security best practices:

- **TLS 1.2+** with hardened cipher suites
- **Client certificate authentication** (mTLS) for multi-tenancy
- **Non-root container** user (distroless base image)
- **Security scanning** in CI (govulncheck, Trivy)
- **Kubernetes security context** (dropped capabilities, seccomp)

See [SECURITY.md](SECURITY.md) for our security policy and reporting vulnerabilities.

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- How to set up your development environment
- Code style guidelines
- Testing requirements
- Pull request process

---

## Project Goals

This project was built to learn and demonstrate:

* Low-level TCP networking with Go
* Designing a line-based protocol from scratch
* Building concurrent systems with minimal external dependencies
* Writing testable and structured code for infrastructure services

---

## Documentation

- [README.md](README.md) - This file
- [SECURITY.md](SECURITY.md) - Security policy and best practices
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [CHANGELOG.md](CHANGELOG.md) - Version history
- [REVIEW_REPORT.md](REVIEW_REPORT.md) - Comprehensive code review report
- [k8s/README.md](k8s/README.md) - Kubernetes deployment guide

---

## Final Notes

This project is a **learning exercise** and not meant for production.
