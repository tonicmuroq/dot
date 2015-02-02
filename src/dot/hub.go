package dot

import (
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"config"
	"types"
	. "utils"
)

const (
	checkAliveDuration = 60 * time.Second
	maxMessageSize     = 1024 * 1024
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024 * 1024,
		WriteBufferSize: 1024 * 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	ZeroTime time.Time
	LeviHub  *Hub
)

type Connection struct {
	ws   *websocket.Conn
	host string
	port int
}

type NInfo struct {
	ID     int
	SubApp string
}

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
		av := types.GetVersionByID(avID)
		if av == nil {
			continue
		}
		app := types.GetApplication(av.Name)
		if app == nil {
			continue
		}

		cg := map[string][]*types.Container{}
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

			localUD := path.Join(config.Config.Nginx.LocalUpDir, fmt.Sprintf("%s.upstream.conf", appname))
			localSD := path.Join(config.Config.Nginx.LocalServerDir, fmt.Sprintf("%s.server.conf", appname))
			remoteUD := path.Join(config.Config.Nginx.RemoteUpDir, fmt.Sprintf("%s.upstream.conf", appname))
			remoteSD := path.Join(config.Config.Nginx.RemoteServerDir, fmt.Sprintf("%s.server.conf", appname))

			cs, exists := cg[appname]
			if !exists {
				// 删
				EnsureFileAbsent(localUD)
				EnsureFileAbsent(localSD)
				if err := exec.Command("res", "nginx_clean", remoteUD).Run(); err != nil {
					Logger.Info("res", "nginx_clean", remoteUD)
					Logger.Info(err)
				}
				if err := exec.Command("res", "nginx_clean", remoteSD).Run(); err != nil {
					Logger.Info("res", "nginx_clean", remoteSD)
					Logger.Info(err)
				}
				continue
			}

			ups := []string{}
			for _, c := range cs {
				if c.Port == 0 {
					continue
				}
				ups = append(ups, fmt.Sprintf("%s:%v", c.Host().IP, c.Port))
			}

			// create upstream
			if err := UpstreamConf(appname, ups, config.Config.Nginx.UpstreamTemplate, localUD); err != nil {
				Logger.Info("failed to create upstream for", appname)
				continue
			}
			if err := exec.Command("res", "nginx_reload", localUD, remoteUD).Run(); err != nil {
				Logger.Info("res", "nginx_reload", localUD, remoteUD)
				Logger.Info(err)
			}

			// create server
			if err := ServerConf(appname, config.Config.PodName, path.Join("/", av.StaticPath()),
				path.Join(config.Config.Nginx.Staticdir, fmt.Sprintf("/%s/%s/", av.Name, av.Version)),
				config.Config.Nginx.ServerTemplate, localSD); err != nil {
				Logger.Info("failed to create server for", appname)
				continue
			}
			if err := exec.Command("res", "nginx_reload", localSD, remoteSD).Run(); err != nil {
				Logger.Info("res", "nginx_reload", localSD, remoteSD)
				Logger.Info(err)
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
	if h := types.GetHostByIP(host); h != nil {
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

func (self *Hub) Dispatch(host string, task *types.Task) error {
	levi, ok := self.levis[host]
	if task == nil {
		return errors.New("task is nil")
	}
	if !ok || levi == nil {
		if job := types.GetJob(task.ID); job != nil {
			job.Done(types.FAIL, "failed cuz no levi alive")
		}
		return errors.New(fmt.Sprintf("%s levi not exists", host))
	}
	levi.inTask <- task
	if task != nil && (task.Type == types.TESTAPPLICATION || task.Type == types.BUILDIMAGE) {
		streamLogHub.GetBufferedLog(task.ID, true)
	}
	return nil
}

func init() {
	LeviHub = &Hub{
		levis:         make(map[string]*Levi),
		lastCheckTime: make(map[string]time.Time),
		apps:          map[int][]string{},
		done:          make(chan *NInfo),
		immediate:     make(chan bool),
		size:          10,
		finished:      false,
	}
}

// Connection methods
func (self *Connection) Ping(payload []byte) error {
	return self.ws.WriteMessage(websocket.PingMessage, payload)
}

func (self *Connection) CloseConnection() error {
	return self.ws.Close()
}

func NewConnection(ws *websocket.Conn, host string, port int) *Connection {
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(ZeroTime)
	ws.SetWriteDeadline(ZeroTime)
	ws.SetPongHandler(func(s string) error {
		LeviHub.lastCheckTime[host] = time.Now()
		Logger.Info("Connection pong: ", s, " from host: ", host)
		return nil
	})
	c := &Connection{ws: ws, host: host, port: port}
	return c
}

func ServeWS(w http.ResponseWriter, r *http.Request) {
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
	LeviHub.AddLevi(levi)
	types.NewHost(ip, "")

	go levi.Run()
	go levi.WaitTask()
}
