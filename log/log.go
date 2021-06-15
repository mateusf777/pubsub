package log

import (
	"fmt"
	logger "log"
)

const logTemplate = "[%s]-%s- %v"

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

type Logger struct {
	context string
	Level   Level
}

func New() Logger {
	return Logger{
		Level: Level{"", -1},
	}
}

func (l Logger) WithContext(context string) Logger {
	l.context = context
	return l
}

func (l Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

func (l Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

func (l Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

func (l Logger) log(level Level, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if level.priority <= l.Level.priority {
		logger.Printf(logTemplate, level, l.context, msg)
	}
}
