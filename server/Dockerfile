FROM golang:1.23.4 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /pubsub ./cmd/pubsub

FROM gcr.io/distroless/static:nonroot

USER nonroot:nonroot

COPY --from=builder /pubsub /pubsub

ENTRYPOINT ["/pubsub"]