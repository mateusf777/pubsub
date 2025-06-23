
cd server
go build -o ../build/ps-server.exe ./cmd/pubsub
cd ../example
go build -o ../build/queue.exe ./queue
go build -o ../build/subscriber.exe ./subscriber
go build -o ../build/publisher.exe ./publisher
go build -o ../build/subscriber_tls ./subscriber_tls
go build -o ../build/publisher_tls ./publisher_tls
go build -o ../build/request.exe ./request
cd ..
