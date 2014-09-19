package main

import (
	"errors"
	"fmt"
	"github.com/CMGS/websocket"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	checkAliveDuration = 60 * time.Second
	writeWait          = 10 * time.Second
	pongWait           = 60 * time.Second
	maxMessageSize     = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 1024,
	WriteBufferSize: 1024 * 1024,
}

var ZeroTime time.Time

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
	appIds        []int
	done          chan int
	immediate     chan bool
	closed        chan bool
	size          int
}

// Hub methods
func (self *Hub) CheckAlive() {
	for {
		for host, last := range self.lastCheckTime {
			duration := time.Since(last)
			// 如果一个连接不再存在, 那么删掉这个连接
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

func (self *Hub) Run() {
	finish := false
	for !finish {
		select {
		case appId := <-self.done:
			self.appIds = append(self.appIds, appId)
			if len(self.appIds) >= self.size {
				logger.Info("restart nginx on full")
				self.RestartNginx()
			}
		case <-self.closed:
			if len(self.appIds) != 0 {
				logger.Info("restart nginx on close")
				self.RestartNginx()
			}
			finish = true
		case <-time.After(time.Second * time.Duration(config.Task.Dispatch)):
			if len(self.appIds) != 0 {
				logger.Info("restart nginx on schedule")
				self.RestartNginx()
			}
		case <-self.immediate:
			if len(self.appIds) != 0 {
				logger.Info("restart nginx immediately")
				self.RestartNginx()
			}
		}
	}
}

func (self *Hub) RestartNginx() {
	for _, appId := range self.appIds {
		if app := GetApplicationById(appId); app != nil {

			conf := path.Join(config.Nginx.Conf, fmt.Sprintf("%s.conf", app.Name))
			var data = struct {
				Name  string
				Hosts []string
			}{
				Name:  app.Name,
				Hosts: []string{},
			}

			hosts := app.Hosts()

			if len(hosts) == 0 {
				EnsureFileAbsent(conf)
			} else {
				f, err := os.Create(conf)
				defer f.Close()
				if err != nil {
					logger.Info("Create nginx conf failed", err)
					continue
				}
				for _, host := range hosts {
					hostStr := fmt.Sprintf("%s:%v", host.IP, config.Nginx.Port)
					data.Hosts = append(data.Hosts, hostStr)
				}
				tmpl := template.Must(template.ParseFiles(config.Nginx.Template))
				if err := tmpl.Execute(f, data); err != nil {
					logger.Info("Render nginx conf failed", err)
				}
			}
		}
	}
	cmd := exec.Command("nginx", "-s", "reload")
	if err := cmd.Run(); err != nil {
		logger.Info("Restart nginx failed", err)
	}
	self.appIds = []int{}
}

func (self *Hub) AddLevi(levi *Levi) {
	host := levi.host
	self.levis[host] = levi
	self.lastCheckTime[host] = time.Now()
}

func (self *Hub) RemoveLevi(host string) {
	levi, ok := self.levis[host]
	if !ok || levi == nil {
		return
	}
	delete(self.levis, host)
	delete(self.lastCheckTime, host)
}

func (self *Hub) Close() {
	for _, levi := range self.levis {
		levi.Close()
	}
	self.closed <- true
}

func (self *Hub) Dispatch(host string, task *Task) error {
	levi, ok := self.levis[host]
	if !ok || levi == nil {
		return errors.New(fmt.Sprintf("%s levi not exists", host))
	}
	if task == nil {
		return errors.New("task is nil")
	}
	levi.inTask <- task
	return nil
}

var hub = &Hub{
	levis:         make(map[string]*Levi),
	lastCheckTime: make(map[string]time.Time),
	appIds:        []int{},
	done:          make(chan int),
	closed:        make(chan bool),
	immediate:     make(chan bool),
	size:          10,
}

// Connection methods
func (self *Connection) Read() ([]byte, error) {
	_, message, err := self.ws.ReadMessage()
	return message, err
}

func (self *Connection) Write(mt int, payload []byte) error {
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

func NewConnection(ws *websocket.Conn, host string, port int) *Connection {
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(ZeroTime)
	ws.SetWriteDeadline(ZeroTime)
	ws.SetPongHandler(func(string) error {
		hub.lastCheckTime[host] = time.Now()
		return nil
	})
	c := &Connection{ws: ws, host: host, port: port, closed: false}
	return c
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
	c := NewConnection(ws, ip, port)
	levi := NewLevi(c, config.Task.Queuesize)
	hub.AddLevi(levi)
	NewHost(ip, "")

	go levi.Run()
	go levi.WaitTask()
}
