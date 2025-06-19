@echo off

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
go clean -testcache
go test . -race

cd ..

cd server
mockery
go clean -testcache
go test . -race

cd ..

cd client
mockery
go clean -testcache
go test . -race

cd ..

start "" /b build\ps-server.exe

start "" /b cmd /c "set PUBSUB_TLS_CERT=%CERT_DIR%\fullchain.pem && set PUBSUB_TLS_KEY=%CERT_DIR%\privkey.pem && set PUBSUB_ADDRESS=0.0.0.0:9443 && build\ps-server.exe"


cd example/integration
go clean -testcache
go test -race .

REM Stop both servers
for /f "tokens=2" %%a in ('tasklist /fi "imagename eq ps-server.exe" /fo list ^| findstr "PID"') do (
    taskkill /PID %%a /F
)

cd ../..