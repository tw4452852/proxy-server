package main

import (
	"log"
)

type DebugLog bool

var (
	Debug DebugLog
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func (d DebugLog) Printf(format string, args ...interface{}) {
	if d {
		log.Printf(format, args...)
	}
}

func (d DebugLog) Println(args ...interface{}) {
	if d {
		log.Println(args...)
	}
}

func (d DebugLog) SetPrefix(prefix string) {
	if d {
		log.SetPrefix(prefix)
	}
}
