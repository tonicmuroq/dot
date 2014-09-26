package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"path"
	"sync"
	"time"
)

type Levi struct {
	conn    *Connection
	inTask  chan *Task
	closed  chan bool
	host    string
	size    int
	tasks   map[string]*GroupedTask
	waiting map[string]*GroupedTask
	wg      *sync.WaitGroup
}

func NewLevi(conn *Connection, size int) *Levi {
	return &Levi{
		conn:    conn,
		inTask:  make(chan *Task),
		closed:  make(chan bool),
		host:    conn.host,
		size:    size,
		tasks:   make(map[string]*GroupedTask),
		waiting: make(map[string]*GroupedTask),
		wg:      &sync.WaitGroup{},
	}
}

func (self *Levi) Host() *Host {
	return GetHostByIP(self.host)
}

func (self *Levi) WaitTask() {
	defer self.wg.Done()
	finish := false
	for !finish {
		select {
		case task, ok := <-self.inTask:
			logger.Debug("levi got task ", task, ok)
			if task == nil {
				// 有nil, 无视掉
				break
			}
			if !ok {
				// 有错, 关掉
				finish = true
			}
			key := fmt.Sprintf("%s:%s:%s", task.Name, task.Uid, task.Type)
			if _, exists := self.tasks[key]; !exists {
				self.tasks[key] = &GroupedTask{
					Name:  task.Name,
					Uid:   task.Uid,
					Type:  task.Type,
					Id:    uuid.New(),
					Tasks: []*Task{},
				}
			}
			self.tasks[key].Tasks = append(self.tasks[key].Tasks, task)
			if self.Len() >= self.size {
				logger.Debug("send tasks")
				self.SendTasks()
			}
		case <-self.closed:
			if self.Len() != 0 {
				logger.Debug("send tasks")
				self.SendTasks()
			}
			finish = true
		case <-time.After(time.Second * time.Duration(config.Task.Dispatch)):
			if self.Len() != 0 {
				logger.Debug("send tasks")
				self.SendTasks()
			}
		}
	}
}

func (self *Levi) Close() {
	self.wg.Add(1)
	self.inTask <- nil
	self.closed <- true
	close(self.inTask)
	close(self.closed)
	self.wg.Wait()
	self.conn.CloseConnection()
}

func (self *Levi) SendTasks() {
	self.wg.Add(len(self.tasks))
	for _, groupedTask := range self.tasks {
		go func(groupedTask *GroupedTask) {
			defer self.wg.Done()
			self.waiting[groupedTask.Id] = groupedTask
			if err := self.conn.ws.WriteJSON(&groupedTask); err != nil {
				logger.Info(err, "JSON write error")
			}
		}(groupedTask)
	}
	self.wg.Wait()
	self.tasks = make(map[string]*GroupedTask)
}

func (self *Levi) Run() {
	// 接收数据
	finish := false
	defer func() {
		self.Close()
		hub.RemoveLevi(self.host)
	}()
	host := self.Host()
	for !finish {
		var taskReply TaskReply
		switch err := self.conn.ws.ReadJSON(&taskReply); {
		case err != nil:
			logger.Info("read json error: ", err)
			finish = true
		case err == nil:
			cleanWaiting := true

			for taskUUID, taskReplies := range taskReply {

				// test if it's special command
				if taskUUID == "__status__" {
					continue
					// do special command
				}
				groupedTask, exists := self.waiting[taskUUID]
				if !exists || (exists && len(groupedTask.Tasks) != len(taskReplies)) {
					logger.Info("task reply is not zippable with tasks, ignore")
					continue
				}
				var app *Application
				for i := 0; i < len(groupedTask.Tasks); i = i + 1 {
					task := groupedTask.Tasks[i]
					retval := taskReplies[i]
					logger.Debug("tasks[i]: ", task)
					logger.Debug("taskReplies[i]: ", retval)
					if task == nil || retval == nil {
						logger.Info("task/retval is nil, ignore")
						continue
					}
					switch task.Type {
					case AddContainer:
						logger.Debug("Add container Feedback")
						app = GetApplicationByNameAndVersion(task.Name, task.Version)
						if app == nil || host == nil {
							logger.Info("app/host 没了")
							continue
						}
						NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
					case RemoveContainer:
						logger.Debug("Remove container Feedback")
						old := GetContainerByCid(task.Container)
						app = old.Application()
						if old == nil || app == nil {
							logger.Info("要删的容器已经不在了")
							continue
						}
						old.Delete()
					case UpdateContainer:
						logger.Debug("Update container Feedback")
						old := GetContainerByCid(task.Container)
						if old != nil {
							old.Delete()
						}
						app = GetApplicationByNameAndVersion(task.Name, task.Version)
						if app == nil || host == nil {
							logger.Info("app/host 没了")
							continue
						}
						NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
					case TestApplication:
						logger.Debug("Test App Feedback")
						// just ignore all feedback
						if v, ok := retval.(string); ok {
							logger.Debug("testing container id is: ", v)
							cleanWaiting = false
						}
					case BuildImage:
						logger.Debug("Build image")
						app = GetApplicationByNameAndVersion(task.Name, task.Version)
						if app == nil {
							logger.Info("app 没了")
							continue
						}
					}
				}

				if NeedToRestartNginx(groupedTask.Type) && app != nil {
					hub.done <- app.Id
				}

				switch groupedTask.Type {
				// case AddContainer:
				// 暂时忽略这个好了, 影响不会太大
				case RemoveContainer:
					if host != nil && len(GetContainerByHostAndApp(host, app)) == 0 {
						hub.immediate <- true
					}
				case BuildImage:
					if app != nil {
						appUserUid := app.UserUid()
						staticPath := path.Join(config.Nginx.Staticdir, app.Name, app.Version)
						staticSrcPath := path.Join(config.Nginx.Staticsrcdir, app.Name, app.Version)
						if err := CopyFiles(staticPath, staticSrcPath, appUserUid, appUserUid); err != nil {
							logger.Info("copy files error: ", err)
						}
					}
				}
				if cleanWaiting {
					delete(self.waiting, taskUUID)
				}
			}
		}
	}
}

func (self *Levi) Len() int {
	count := 0
	for _, value := range self.tasks {
		count = count + len(value.Tasks)
	}
	return count
}

func NeedToRestartNginx(taskType int) bool {
	return taskType == AddContainer || taskType == RemoveContainer || taskType == UpdateContainer
}
