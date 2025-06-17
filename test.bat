@echo off

cd server &&
go build -o ../build/ps-server ./cmd/pubsub
cd ../example &&
go build -o ../build/queue ./queue &&
go build -o ../build/subscriber ./subscriber &&
go build -o ../build/publisher ./publisher &&
go build -o ../build/request ./request

cd ..

docker build -f server/Dockerfile -t pubsub:latest .

cd core &&
mockery &&
go clean -testcache
go test . -race

cd ..

cd server &&
mockery &&
go clean -testcache
go test . -race

cd ..

cd client &&
mockery &&
go clean -testcache
go test . -race

cd ..

docker run -d --rm -p 9999:9999 --name test pubsub:latest
docker run -d --rm ^
    -e PUBSUB_TLS_CERT=/fullchain.pem ^
    -e PUBSUB_TLS_KEY=/privkey.pem ^
    -e PUBSUB_ADDRESS=0.0.0.0:9443 ^
    -p 9443:9443 ^
    -v %CERT_DIR%\fullchain.pem:/fullchain.pem ^
    -v %CERT_DIR%\privkey.pem:/privkey.pem ^
    --name test-tls pubsub:latest

cd example/integration
go clean -testcache
go test -v . -race
docker stop test
docker stop test-tls

cd ../..