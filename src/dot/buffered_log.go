package dot

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"

	. "utils"
)

type BufferedLog struct {
	sync.Mutex
	Id          int
	input       chan string
	buffer      []string
	connections []*websocket.Conn
	running     bool
}

type StreamLogHub map[int]*BufferedLog

func (self StreamLogHub) GetBufferedLog(id int, create bool) *BufferedLog {
	b, exists := self[id]
	if !exists {
		if !create {
			return nil
		}
		b = NewBufferedLog(id)
		go b.Run()
		self[id] = b
	}
	return b
}

func (self StreamLogHub) RemoveBufferedLog(id int) {
	b, exists := self[id]
	if !exists {
		return
	}
	b.Stop()
	delete(self, id)
}

var (
	streamLogHub = StreamLogHub{}
	closeMessage = websocket.FormatCloseMessage(websocket.CloseMessage, "close")
)

func writeLogLine(w *websocket.Conn, line string) {
	err := w.WriteMessage(websocket.TextMessage, []byte(line))
	if err != nil {
		w.Close()
	}
}

func NewBufferedLog(id int) *BufferedLog {
	return &BufferedLog{
		Id:          id,
		input:       make(chan string),
		buffer:      []string{},
		connections: []*websocket.Conn{},
		running:     true,
	}
}

func (self *BufferedLog) AddWebsocket(ws *websocket.Conn) {
	self.connections = append(self.connections, ws)
	self.Lock()
	defer self.Unlock()
	for _, line := range self.buffer {
		writeLogLine(ws, line)
	}
}

func (self *BufferedLog) Run() {
	var line string
	for self.running {
		select {
		case line, self.running = <-self.input:
			self.broadcastLine(line)
		}
	}
}

func (self *BufferedLog) broadcastLine(line string) {
	self.Lock()
	defer self.Unlock()
	self.buffer = append(self.buffer, line)
	for _, ws := range self.connections {
		writeLogLine(ws, line)
	}
}

func (self *BufferedLog) Stop() {
	close(self.input)
	for _, w := range self.connections {
		w.WriteMessage(websocket.CloseMessage, closeMessage)
		w.Close()
	}
}

func (self *BufferedLog) Feed(line string) {
	self.input <- line
}

func ServeLogWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Logger.Info(err)
		return
	}
	r.ParseForm()
	taskId := r.Form.Get("task")
	id, _ := strconv.Atoi(taskId)
	b := streamLogHub.GetBufferedLog(id, false)
	if b == nil {
		http.Error(w, "Wrong Task ID", 400)
		return
	}
	b.AddWebsocket(ws)
}
