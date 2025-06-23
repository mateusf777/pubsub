#!/usr/bin/sh

cd server
go build -o ../build/ps-server ./cmd/pubsub
cd ../example
go build -o ../build/queue ./queue
go build -o ../build/subscriber ./subscriber
go build -o ../build/publisher ./publisher
go build -o ../build/subscriber_tls ./subscriber_tls
go build -o ../build/publisher_tls ./publisher_tls
go build -o ../build/request ./request
cd ..
