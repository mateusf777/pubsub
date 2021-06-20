#!/usr/bin/sh

go build -o ./build/ps-server .  &&
  go build -o ./build/queue ./example/queue &&
  go build -o ./build/subscriber ./example/subscriber &&
  go build -o ./build/publisher ./example/publisher &&
  go build -o ./build/request ./example/request &&
  go build -o ./build/cli ./example/cli