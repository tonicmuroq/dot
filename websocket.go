package main

import (
	"github.com/CMGS/websocket"
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
	levis         map[string]*Levi
	lastCheckTime map[string]time.Time
}

// Hub methods
func (self *Hub) CheckAlive() {
	for {
		for host, last := range self.lastCheckTime {
			duration := time.Since(last)
			// 类型真恶心, 自动转换会死啊
			// 如果一个连接不再存在, 那么先关闭连接, 再删掉这个连接
			if duration.Seconds() > float64(checkAliveDuration) {
				logger.Info(host, " is disconnected.")
				self.RemoveLevi(host)
			}
		}
		for host, levi := range self.levis {
			levi.conn.Ping([]byte(host))
		}
		time.Sleep(checkAliveDuration)
	}
}

func (self *Hub) AddLevi(levi *Levi) {
	host := levi.host
	self.levis[host] = levi
	self.lastCheckTime[host] = time.Now()
}

func (self *Hub) GetLevi(host string) *Levi {
	return self.levis[host]
}

func (self *Hub) RemoveLevi(host string) {
	levi, ok := self.GetLevi(host)
	if !ok {
		return
	}
	levi.Close()
	delete(self.levis, host)
	delete(self.lastCheckTime, host)
}

func (self *Hub) Close() {
	for _, levi := range self.levis {
		levi.Close()
	}
}

func (self *Hub) Dispatch(host string, task *Task) {
	levi, ok := self.GetLevi(host)
	if !ok {
		logger.Info("Not exists")
		return
	}
	levi.inTask <- task
}

var hub = &Hub{
	levis:         make(map[string]*Levi),
	lastCheckTime: make(map[string]time.Time),
}

func (self *Connection) Init() {
	self.ws.SetReadLimit(maxMessageSize)
	self.ws.SetPongHandler(func(string) error {
		self.ws.SetReadDeadline(time.Now().Add(pongWait))
		hub.lastCheckTime[self.host] = time.Now()
		return nil
	})
}

// Connection methods
func (self *Connection) Read() ([]byte, error) {
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
	defer hub.RemoveLevi(self.host)
	for !self.closed {
		msg, err := self.Read()
		if err != nil {
			logger.Info(err, "Listen")
			self.CloseConnection()
		}
		// TODO action
		self.Write(websocket.TextMessage, []byte(msg))
	}
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
		logger.Info(err)
		return
	}

	// 创建个新连接, 新建一条host记录
	// 同时开始 listen
	c := &Connection{ws: ws, host: ip, port: port, closed: false}
	c.Init()
	levi := NewLevi(c, config.Task.Queuesize)
	hub.AddLevi(levi)
	NewHost(ip, "")

	go levi.Run()
	go levi.WaitTask()
}
