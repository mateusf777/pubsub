# -----------------------------------------------------------------------------
# Dockerfile for building the pubsub server binary and packaging it in a minimal
# distroless container. This image is primarily used for integration testing,
# but can also be used to run the server in production.
#
# Expects configuration via environment variables (see main.go for details).
# -----------------------------------------------------------------------------
FROM golang:1.23.4 AS builder

WORKDIR /app

# Copy the entire repo to ensure local replace directives work
COPY . ./

WORKDIR /app/server

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /pubsub ./cmd/pubsub

FROM gcr.io/distroless/static:nonroot

USER nonroot:nonroot

COPY --from=builder /pubsub /pubsub

ENTRYPOINT ["/pubsub"]