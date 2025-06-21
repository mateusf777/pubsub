@echo off

go clean -testcache

cd server
go build -o ../build/ps-server.exe ./cmd/pubsub
cd ../example
go build -o ../build/queue.exe ./queue
go build -o ../build/subscriber.exe ./subscriber
go build -o ../build/publisher.exe ./publisher
go build -o ../build/request.exe ./request

cd ..

cd core
mockery
go test . -race

cd ..

cd server
mockery
go test . -race

cd ..

cd client
mockery
go test . -race

cd ..
cd example/integration

start "" /b build\ps-server.exe

go test -race -run '^(TestPublish|TestQueue|TestRequest)$' .

for /f "tokens=2" %%a in ('tasklist /fi "imagename eq ps-server.exe" /fo list ^| findstr "PID"') do (
    taskkill /PID %%a /F
)

start "" /b cmd /c "set PUBSUB_TLS_CERT=%CERT_DIR%\fullchain.pem && set PUBSUB_TLS_KEY=%CERT_DIR%\privkey.pem && set PUBSUB_ADDRESS=0.0.0.0:9443 && build\ps-server.exe"

go test -race -run '^(TestPublishTLS|TestQueueTLS|TestRequestTLS)$' .

for /f "tokens=2" %%a in ('tasklist /fi "imagename eq ps-server.exe" /fo list ^| findstr "PID"') do (
    taskkill /PID %%a /F
)

start "" /b cmd /c "set PUBSUB_TLS_CERT=%CERT_DIR%\fullchain.pem && set PUBSUB_TLS_KEY=%CERT_DIR%\privkey.pem && set PUBSUB_TLS_CA=%CERT_DIR%\myca-cert.pem && set PUBSUB_ADDRESS=0.0.0.0:9443 && build\ps-server.exe"

go test -race -run '^(TestConnectTLSNoCert|TestConnectTLSInvalidCert|TestConnectTLS)$' .

for /f "tokens=2" %%a in ('tasklist /fi "imagename eq ps-server.exe" /fo list ^| findstr "PID"') do (
    taskkill /PID %%a /F
)

cd ../..