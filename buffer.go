package main

import (
	"github.com/gorilla/websocket"
	"sync"
)

type Streamer struct {
	sync.Mutex
	Id          int
	input       chan string
	buffer      []string
	connections []*websocket.Conn
	running     bool
}

func writeLogLine(w *websocket.Conn, line string) {
	if err := w.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
		w.Close()
	}
}

func NewStreamer(id int) *Streamer {
	return &Streamer{
		Id:          id,
		input:       make(chan string),
		buffer:      []string{},
		connections: []*websocket.Conn{},
		running:     true,
	}
}

func (self *Streamer) AddWebsocket(ws *websocket.Conn) {
	self.connections = append(self.connections, ws)
	self.Lock()
	for _, line := range self.buffer {
		writeLogLine(ws, line)
	}
	self.Unlock()
}

func (self *Streamer) Run() {
	var line string
	for self.running {
		select {
		case line, self.running = <-self.input:
			self.Lock()
			self.buffer = append(self.buffer, line)
			for _, ws := range self.connections {
				writeLogLine(ws, line)
			}
			self.Unlock()
		}
	}
}

func (self *Streamer) Stop() {
	self.running = false
	close(self.input)
	for _, w := range self.connections {
		w.Close()
	}
}

func (self *Streamer) Feed(line string) {
	self.input <- line
}
