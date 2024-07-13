package main

import (
	"fmt"

	g "xabbo.b7c.io/goearth"
)

var isLoggingEnabled bool

// SetupPacketLogging initializes packet logging for the extension
func SetupPacketLogging(ext *g.Ext) {
	ext.InterceptAll(func(e *g.Intercept) {
		if !isLoggingEnabled {
			return
		}
		if e.Packet.Header.Dir == g.Out {
			fmt.Printf("Outgoing packet: %s, Data: %v\n", ext.Headers().Name(e.Packet.Header), e.Packet.Data)
		} else {
			fmt.Printf("Incoming packet: %s, Data: %v\n", ext.Headers().Name(e.Packet.Header), e.Packet.Data)
		}
	})
}

// EnableLogging turns on packet logging
func EnableLogging() {
	isLoggingEnabled = true
}

// DisableLogging turns off packet logging
func DisableLogging() {
	isLoggingEnabled = false
}
