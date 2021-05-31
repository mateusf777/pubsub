package main

import (
	"fmt"
	"log"
)

const logTemplate = "[%s] %v"

type Level string

const (
	ERR   Level = "ERR"
	INFO  Level = "INFO"
	DEBUG Level = "DEBUG"
)

func Log(level Level, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Printf(logTemplate, level, msg)
}
