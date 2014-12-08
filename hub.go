package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	"./config"
	"./models"
	. "./utils"

	"github.com/gorilla/websocket"
)

const (
	checkAliveDuration = 60 * time.Second
	maxMessageSize     = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 1024,
	WriteBufferSize: 1024 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
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
	size          int
	finished      bool
}

// Hub methods
func (self *Hub) CheckAlive() {
	for !self.finished {
		for host, last := range self.lastCheckTime {
			duration := time.Since(last)
			// 如果一个连接不再存在, 那么删掉这个连接
			if duration.Seconds() > float64(checkAliveDuration) {
				Logger.Info(host, " is disconnected.")
				if h := models.GetHostByIP(host); h != nil {
					h.Offline()
				}
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
	for !self.finished {
		select {
		case appId := <-self.done:
			self.appIds = append(self.appIds, appId)
			if len(self.appIds) >= self.size {
				Logger.Info("restart nginx on full")
				self.RestartNginx()
			}
		case <-time.After(time.Second * time.Duration(config.Config.Task.Dispatch)):
			if len(self.appIds) != 0 {
				Logger.Info("restart nginx on schedule")
				self.RestartNginx()
			}
		case <-self.immediate:
			if len(self.appIds) != 0 {
				Logger.Info("restart nginx immediately")
				self.RestartNginx()
			}
		}
	}
}

func (self *Hub) RestartNginx() {
	for _, appId := range self.appIds {
		if app := models.GetApplicationById(appId); app != nil {

			conf := path.Join(config.Config.Nginx.Conf, fmt.Sprintf("%s.conf", app.Name))
			var data = struct {
				Name  string
				Path  string
				Hosts []string
			}{
				Name:  app.Name,
				Path:  path.Join(config.Config.Nginx.Staticdir, fmt.Sprintf("/%s/%s/", app.Name, app.Version)),
				Hosts: []string{},
			}

			hosts := app.AllVersionHosts()

			if len(hosts) == 0 {
				EnsureFileAbsent(conf)
			} else {
				f, err := os.Create(conf)
				defer f.Close()
				if err != nil {
					Logger.Info("Create nginx conf failed", err)
					continue
				}
				for _, host := range hosts {
					hostStr := fmt.Sprintf("%s:%v", host.IP, config.Config.Nginx.Port)
					data.Hosts = append(data.Hosts, hostStr)
				}
				tmpl := template.Must(template.ParseFiles(config.Config.Nginx.Template))
				if err := tmpl.Execute(f, data); err != nil {
					Logger.Info("Render nginx conf failed", err)
				}
			}

			app.CreateDNS()
		}
	}
	cmd := exec.Command("nginx", "-s", "reload")
	if err := cmd.Run(); err != nil {
		Logger.Info("Restart nginx failed", err)
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
	self.finished = true
}

func (self *Hub) Dispatch(host string, task *models.Task) error {
	levi, ok := self.levis[host]
	if task == nil {
		return errors.New("task is nil")
	}
	if !ok || levi == nil {
		if st := models.GetStoredTaskById(task.Id); st != nil {
			st.Done(models.FAIL, "failed cuz no levi alive")
		}
		return errors.New(fmt.Sprintf("%s levi not exists", host))
	}
	levi.inTask <- task
	if task != nil && (task.Type == models.TestApplication || task.Type == models.BuildImage) {
		streamLogHub.GetBufferedLog(task.Id, true)
	}
	return nil
}

var hub = &Hub{
	levis:         make(map[string]*Levi),
	lastCheckTime: make(map[string]time.Time),
	appIds:        []int{},
	done:          make(chan int),
	immediate:     make(chan bool),
	size:          10,
	finished:      false,
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
		Logger.Info(err)
		return
	}

	// 创建个新连接, 新建一条host记录
	// 同时开始 listen
	c := NewConnection(ws, ip, port)
	levi := NewLevi(c, config.Config.Task.Queuesize)
	hub.AddLevi(levi)
	models.NewHost(ip, "")

	go levi.Run()
	go levi.WaitTask()
}
