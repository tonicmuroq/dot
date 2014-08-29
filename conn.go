package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	checkAliveDuration = 60 * time.Second
	writeWait          = 10 * time.Second
	pongWait           = 60 * time.Second
	maxMessageSize     = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// websocket 连接
type Connection struct {
	ws   *websocket.Conn
	host string
	port int
}

// 保存所有连接, 定时 ping
type Hub struct {
	connections   map[string]*Connection
	using         map[string]bool
	lastCheckTime map[string]time.Time
}

// Hub methods
func (self *Hub) checkAlive() {
	for {
		for host, last := range self.lastCheckTime {
			duration := time.Now().Sub(last)
			// 类型真恶心, 自动转换会死啊
			// 如果一个连接不再存在, 那么先关闭连接, 再删掉这个连接
			if duration.Seconds() > float64(checkAliveDuration) {
				log.Println(host, " is disconnected.")
				conn, err := self.connections[host]
				if err {
					conn.closeConnection()
				}
				delete(self.connections, host)
				delete(self.lastCheckTime, host)
				delete(self.using, host)
			}
		}
		for host, conn := range self.connections {
			conn.ping([]byte(host))
		}
		time.Sleep(checkAliveDuration)
	}
}

func (self *Hub) addConnection(conn *Connection) {
	host := conn.host
	self.connections[host] = conn
	self.using[host] = false
	self.lastCheckTime[host] = time.Now()
}

func (self *Hub) getConnection(host string) (*Connection, bool) {
	conn, err := self.connections[host]
	if !err {
		return nil, false
	}
	if !self.using[host] {
		self.using[host] = true
		return conn, true
	} else {
		return nil, false
	}
}

func (self *Hub) putConnection(conn *Connection) {
	host := conn.host
	if self.using[host] {
		self.using[host] = false
	}
}

var hub = &Hub{
	connections:   make(map[string]*Connection),
	using:         make(map[string]bool),
	lastCheckTime: make(map[string]time.Time),
}

// Connection methods
func (self *Connection) read() []byte {
	self.ws.SetReadLimit(maxMessageSize)
	self.ws.SetPongHandler(func(string) error {
		self.ws.SetReadDeadline(time.Now().Add(pongWait))
		log.Println("这里应该更新数据")
		hub.lastCheckTime[self.host] = time.Now()
		return nil
	})
	_, message, err := self.ws.ReadMessage()
	if err != nil {
		log.Println(err)
		return []byte{}
	}
	return message
}

func (self *Connection) write(mt int, payload []byte) error {
	self.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return self.ws.WriteMessage(mt, payload)
}

func (self *Connection) ping(payload []byte) error {
	return self.write(websocket.PingMessage, payload)
}

func (self *Connection) send(payload []byte) error {
	return self.write(websocket.TextMessage, payload)
}

func (self *Connection) closeConnection() error {
	return self.ws.Close()
}

func (self *Connection) listen() {
	for {
		msg := self.read()
		log.Println("这是listen读到的东西 ", string(msg))
		self.write(websocket.TextMessage, []byte(msg))
	}
}

func ServeWs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	rs := strings.Split(r.RemoteAddr, ":")
	port, _ := strconv.Atoi(rs[1])
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	c := &Connection{ws: ws, host: rs[0], port: port}
	hub.addConnection(c)
	go c.listen()
}
