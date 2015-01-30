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

type NInfo struct {
	ID     int
	SubApp string
}

// 保存所有连接, 定时 ping
type Hub struct {
	levis         map[string]*Levi
	lastCheckTime map[string]time.Time
	apps          map[int][]string
	done          chan *NInfo
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
				Logger.Info(" disconnected: ", host)
				self.RemoveLevi(host)
			}
		}
		for host, levi := range self.levis {
			levi.conn.Ping([]byte(host))
			Logger.Info(" check alive: ", host)
		}
		time.Sleep(checkAliveDuration)
	}
}

func (self *Hub) Run() {
	for !self.finished {
		select {
		case nInfo := <-self.done:
			self.apps[nInfo.ID] = append(self.apps[nInfo.ID], nInfo.SubApp)
			if len(self.apps) >= self.size {
				Logger.Info("restart nginx on full")
				self.RestartNginx()
			}
		case <-time.After(time.Second * time.Duration(config.Config.Task.Dispatch)):
			if len(self.apps) != 0 {
				Logger.Info("restart nginx on schedule")
				self.RestartNginx()
			}
		case <-self.immediate:
			if len(self.apps) != 0 {
				Logger.Info("restart nginx immediately")
				self.RestartNginx()
			}
		}
	}
}

func (self *Hub) RestartNginx() {
	for avID, subnames := range self.apps {
		av := models.GetVersionByID(avID)
		if av == nil {
			continue
		}
		app := models.GetApplication(av.Name)
		if app == nil {
			continue
		}

		cg := map[string][]*models.Container{}
		for _, c := range app.Containers() {
			if c.SubApp == "" {
				cg[c.AppName] = append(cg[c.AppName], c)
			} else {
				cg[c.SubApp] = append(cg[c.SubApp], c)
			}
		}

		for _, subname := range subnames {
			appname := subname
			if appname == "" {
				appname = app.Name
			}
			conf := path.Join(config.Config.Nginx.Conf, fmt.Sprintf("%s.conf", appname))
			remoteConfig := fmt.Sprintf("/etc/nginx/conf.d/%s.conf", appname)
			var data = struct {
				Name      string
				PodName   string
				Static    string
				Path      string
				UpStreams []string
			}{
				Name:      appname,
				PodName:   config.Config.PodName,
				Static:    path.Join("/", av.StaticPath()),
				Path:      path.Join(config.Config.Nginx.Staticdir, fmt.Sprintf("/%s/%s/", av.Name, av.Version)),
				UpStreams: []string{},
			}

			cs, exists := cg[appname]
			if !exists {
				// 删
				EnsureFileAbsent(conf)
				if err := exec.Command("res", "nginx_clean", remoteConfig).Run(); err != nil {
					Logger.Info("res", "nginx_clean", remoteConfig)
					Logger.Info(err)
				}
			} else {
				f, err := os.Create(conf)
				defer f.Close()
				if err != nil {
					Logger.Info("Create nginx conf failed", err)
					continue
				}
				for _, container := range cs {
					if container.Port == 0 {
						// ignore daemon
						continue
					}
					upStream := fmt.Sprintf("%s:%v", container.Host().IP, container.Port)
					data.UpStreams = append(data.UpStreams, upStream)
				}
				tmpl := template.Must(template.ParseFiles(config.Config.Nginx.Template))
				if err := tmpl.Execute(f, data); err != nil {
					Logger.Info("Render nginx conf failed", err)
				}
				if err := exec.Command("res", "nginx_reload", conf, remoteConfig).Run(); err != nil {
					Logger.Info("res", "nginx_reload", conf, remoteConfig)
					Logger.Info(err)
				}
			}

		}

		app.CreateDNS()
	}
	cmd := exec.Command("nginx", "-s", "reload")
	if err := cmd.Run(); err != nil {
		Logger.Info("Restart nginx failed", err)
	}
	self.apps = map[int][]string{}
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
	if h := models.GetHostByIP(host); h != nil {
		h.Offline()
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
		if job := models.GetJob(task.ID); job != nil {
			job.Done(models.FAIL, "failed cuz no levi alive")
		}
		return errors.New(fmt.Sprintf("%s levi not exists", host))
	}
	levi.inTask <- task
	if task != nil && (task.Type == models.TESTAPPLICATION || task.Type == models.BUILDIMAGE) {
		streamLogHub.GetBufferedLog(task.ID, true)
	}
	return nil
}

var hub = &Hub{
	levis:         make(map[string]*Levi),
	lastCheckTime: make(map[string]time.Time),
	apps:          map[int][]string{},
	done:          make(chan *NInfo),
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
	ws.SetPongHandler(func(s string) error {
		hub.lastCheckTime[host] = time.Now()
		Logger.Info("Connection pong: ", s, " from host: ", host)
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
