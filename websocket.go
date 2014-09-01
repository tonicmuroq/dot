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
	ws     *websocket.Conn
	host   string
	port   int
	closed bool
}

// 保存所有连接, 定时 ping
type Hub struct {
	connections   map[string]*Connection
	using         map[string]bool
	lastCheckTime map[string]time.Time
}

// Hub methods
func (self *Hub) CheckAlive() {
	for {
		for host, last := range self.lastCheckTime {
			duration := time.Now().Sub(last)
			// 类型真恶心, 自动转换会死啊
			// 如果一个连接不再存在, 那么先关闭连接, 再删掉这个连接
			if duration.Seconds() > float64(checkAliveDuration) {
				log.Println(host, " is disconnected.")
				conn, err := self.connections[host]
				if err {
					conn.CloseConnection()
				}
				delete(self.connections, host)
				delete(self.lastCheckTime, host)
				delete(self.using, host)
			}
		}
		for host, conn := range self.connections {
			conn.Ping([]byte(host))
		}
		time.Sleep(checkAliveDuration)
	}
}

func (self *Hub) AddConnection(conn *Connection) {
	host := conn.host
	self.connections[host] = conn
	self.using[host] = false
	self.lastCheckTime[host] = time.Now()
}

func (self *Hub) GetConnection(host string) (*Connection, bool) {
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

func (self *Hub) PutConnection(conn *Connection) {
	host := conn.host
	if self.using[host] {
		self.using[host] = false
	}
}

func (self *Hub) RemoveConenction(conn *Connection) {
	host := conn.host
	delete(self.connections, host)
	delete(self.lastCheckTime, host)
	delete(self.using, host)
}

func (self *Hub) Close() {
	for _, conn := range self.connections {
		conn.CloseConnection()
	}
}

var hub = &Hub{
	connections:   make(map[string]*Connection),
	using:         make(map[string]bool),
	lastCheckTime: make(map[string]time.Time),
}

// Connection methods
func (self *Connection) Read() ([]byte, error) {
	self.ws.SetReadLimit(maxMessageSize)
	self.ws.SetPongHandler(func(string) error {
		self.ws.SetReadDeadline(time.Now().Add(pongWait))
		hub.lastCheckTime[self.host] = time.Now()
		return nil
	})
	_, message, err := self.ws.ReadMessage()
	return message, err
}

func (self *Connection) Write(mt int, payload []byte) error {
	self.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return self.ws.WriteMessage(mt, payload)
}

func (self *Connection) Ping(payload []byte) error {
	return self.Write(websocket.PingMessage, payload)
}

func (self *Connection) Send(payload []byte) error {
	return self.Write(websocket.TextMessage, payload)
}

func (self *Connection) CloseConnection() error {
	self.closed = true
	return self.ws.Close()
}

func (self *Connection) Listen() {
	for !self.closed {
		msg, err := self.Read()
		if err != nil {
			log.Println("出错了, 那么退出这个goroutine吧")
			self.CloseConnection()
		}
		// action
		self.Write(websocket.TextMessage, []byte(msg))
	}
	defer func() {
		hub.RemoveConenction(self)
		log.Println("连接关闭", self)
	}()
}

func ServeWs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// 拿 ip:port
	rs := strings.Split(r.RemoteAddr, ":")
	ip := rs[0]
	port, _ := strconv.Atoi(rs[1])

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// 创建个新连接, 新建一条host记录
	// 同时开始 listen
	c := &Connection{ws: ws, host: ip, port: port, closed: false}
	hub.AddConnection(c)
	NewHost(ip, "")

	go c.Listen()
}
