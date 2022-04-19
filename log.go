package main

import (
	"fmt"
	"log"
)

const (
	debugLevel   uint = 0
	infoLevel    uint = 10
	warningLevel uint = 20
	errorLevel   uint = 30
	fatalLevel   uint = 40
)

// Log provides simple logging functionality
type Log struct {
	Level  uint
	Prefix string
}

func (l *Log) log(lvl uint, msg string, args ...interface{}) bool {
	prefix := l.Prefix

	if prefix != "" {
		prefix += ": "
	}

	if len(args) == 0 {
		log.Printf("%s%s", prefix, msg)
	} else {
		log.Printf(fmt.Sprintf("%s%s", prefix, msg), args...)
	}

	return true
}

// Debug messages
func (l *Log) Debug(msg string, args ...interface{}) bool {
	if l.Level > debugLevel {
		return false
	}

	return l.log(debugLevel, msg, args...)
}

// Error messages
func (l *Log) Error(msg string, args ...interface{}) bool {
	if l.Level > errorLevel {
		return false
	}

	return l.log(errorLevel, msg, args...)
}

// Fatal messages
func (l *Log) Fatal(msg string, args ...interface{}) bool {
	if l.Level > fatalLevel {
		return false
	}

	return l.log(debugLevel, msg, args...)
}

// Info messages
func (l *Log) Info(msg string, args ...interface{}) bool {
	if l.Level > infoLevel {
		return false
	}

	return l.log(debugLevel, msg, args...)
}

// Warning messages
func (l *Log) Warning(msg string, args ...interface{}) bool {
	if l.Level > warningLevel {
		return false
	}

	return l.log(debugLevel, msg, args...)
}
