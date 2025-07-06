#!/bin/bash

# To run this:
: <<'END'
CERT_DIR=/path/to/cert \
PUBSUB_TLS_CERT=$CERT_DIR/fullchain.pem \
PUBSUB_TLS_KEY=$CERT_DIR/privkey.pem \
PUBSUB_TLS_CA=$CERT_DIR/myca-cert.pem \
PUBSUB_TLS_CLIENT_CERT=$CERT_DIR/client-cert.pem \
PUBSUB_TLS_CLIENT_KEY=$CERT_DIR/client-key.pem \
PUBSUB_TLS_INVALID_CLIENT_CERT=$CERT_DIR/invalid-client-cert.pem \
PUBSUB_TLS_INVALID_CLIENT_KEY=$CERT_DIR/invalid-client-key.pem \
PUBSUB_TLS_CLIENT_CERT_B=$CERT_DIR/client-b-cert.pem \
PUBSUB_TLS_CLIENT_KEY_B=$CERT_DIR/client-b-key.pem \
./test.sh
END

go clean -testcache

cd server &&
go build -o ../build/ps-server ./cmd/pubsub
cd ../example &&
go build -o ../build/queue ./queue &&
go build -o ../build/subscriber ./subscriber &&
go build -o ../build/publisher ./publisher &&
go build -o ../build/request ./request

cd ..

docker build -f server/Dockerfile -t pubsub:latest .
docker run -d --rm -p 9999:9999 --name test pubsub:latest

cd core &&
mockery &&
go test . -race

cd ..

cd server &&
mockery &&
go test . -race

cd ..

cd client &&
mockery &&
go test . -race

cd ..
cd example/integration

go test -race -run '^(TestPublish|TestQueue|TestRequest)$' .
docker stop test

docker run -d --rm \
    -e PUBSUB_TLS_CERT=/fullchain.pem \
    -e PUBSUB_TLS_KEY=/privkey.pem \
    -e PUBSUB_ADDRESS=0.0.0.0:9443 \
    -p 9443:9443 \
    -v "$CERT_DIR/fullchain.pem":/fullchain.pem \
    -v "$CERT_DIR/privkey.pem":/privkey.pem \
    --name test-tls pubsub:latest

go test -race -run '^(TestPublishTLS|TestQueueTLS|TestRequestTLS)$' .
docker stop test-tls

docker run -d --rm \
    -e PUBSUB_TLS_CERT=/fullchain.pem \
    -e PUBSUB_TLS_KEY=/privkey.pem \
    -e PUBSUB_TLS_CA=/myca-cert.pem \
    -e PUBSUB_ADDRESS=0.0.0.0:9443 \
    -p 9443:9443 \
    -v "$CERT_DIR/fullchain.pem":/fullchain.pem \
    -v "$CERT_DIR/privkey.pem":/privkey.pem \
    -v "$CERT_DIR/myca-cert.pem":/myca-cert.pem \
    --name test-tls-ca pubsub:latest

go test -race -run '^(TestConnectTLSNoCert|TestConnectTLSInvalidCert|TestConnectTLS|TestPublishDifferentTenants|TestPublishSameTenants)$' .
docker stop test-tls-ca

cd ../..