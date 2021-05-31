package log

import (
	"fmt"
	logger "log"
)

const logTemplate = "[%s] %v"

type Level struct {
	value    string
	priority int
}

func (l Level) String() string {
	return l.value
}

var (
	ERROR = Level{"ERROR", 0}
	INFO  = Level{"INFO", 1}
	DEBUG = Level{"DEBUG", 2}
)

var logLevel = DEBUG

func SetLevel(level Level) {
	logLevel = level
}

func log(level Level, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if level.priority <= logLevel.priority {
		logger.Printf(logTemplate, level, msg)
	}
}

func Debug(format string, v ...interface{}) {
	log(DEBUG, format, v...)
}

func Info(format string, v ...interface{}) {
	log(INFO, format, v...)
}

func Error(format string, v ...interface{}) {
	log(ERROR, format, v...)
}
