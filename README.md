# PUBSUB

## Overview
This project is a **learning-oriented** implementation of a basic pub/sub server and protocol. It was developed to gain a deeper understanding of how pub/sub systems work at a fundamental level.

### **Disclaimer**
This project was inspired by the [NATS protocol](https://docs.nats.io/nats-protocol/nats-protocol#protocol-messages). While it follows similar operations, it is not a replacement for NATS, nor does it aim to be production-ready. If you need a robust pub/sub system for real-world use, consider using [NATS](https://nats.io).

### **Security Warning**
🚨 **This implementation is under development. Authentication and TLS security features are ongoing.** Use it for learning and experimentation only.

---

## **Build Instructions**

Run the following commands to build the server and example applications:

```
cd server
go build -o ../build/ps-server ./cmd/pubsub

cd ../example
go build -o ../build/queue ./queue
go build -o ../build/subscriber ./subscriber
go build -o ../build/publisher ./publisher
go build -o ../build/request ./request

cd ../build
```

---

## **Running the Server**
To start the pub/sub server:

```
./ps-server
```

---

## **Usage Examples**

### **1. Subscribe and Publish Example**
#### Start a Subscriber
```
./subscriber
```

#### Send a Request from Another Terminal
```
./request
```
##### Example Output
```
{"time":"<timestamp>","level":"INFO","msg":"request time"}
{"time":"<timestamp>","level":"INFO","msg":"now","data":"<formatted time>"}
```

#### Launch a Queue Subscriber
```
./queue
```
##### Example Output
```
{"time":"<timestamp>","level":"INFO","msg":"Launching subscribers","queue":3}
{"time":"<timestamp>","level":"INFO","msg":"Queue, successfully subscribed","queue":0}
{"time":"<timestamp>","level":"INFO","msg":"Queue, successfully subscribed","queue":1}
{"time":"<timestamp>","level":"INFO","msg":"Queue, successfully subscribed","queue":2}
...
{"time":"<timestamp>","level":"INFO","msg":"Received all messages"}
```

#### Publish Messages
```
./publisher
```
##### Example Output
```
{"time":"<timestamp>","level":"INFO","msg":"Sending messages","count":10000}
{"time":"<timestamp>","level":"INFO","msg":"Done"}
{"time":"<timestamp>","level":"INFO","msg":"Connection closed"}
```

##### Example Output in Subscriber Terminal
```
{"time":"<timestamp>","level":"INFO","msg":"received","count":10000}
```

---

## **2. Simple Subscribe/Publish via Telnet**
You can interact with the pub/sub server using `telnet`.

### **Start the Server**
```
./ps-server
```

### **Subscribe to a Topic**
```
telnet localhost 9999
```
Then type:
```
sub test 1
```

### **Publish a Message**
Open another terminal and connect via telnet:
```
telnet localhost 9999
```
Then type:
```
pub test
test
```

### **Example Output (Subscriber Terminal)**
```
MSG test 1
test
```

### **Stopping the Server**
To quit in both terminals, type:
```
stop
```

---

## **Final Notes**
This project is a **learning exercise** and not meant for production.
