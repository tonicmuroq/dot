package main

import (
	"fmt"
	"path"
	"sync"
	"time"

	"code.google.com/p/go-uuid/uuid"
)

type Levi struct {
	conn      *Connection
	inTask    chan *Task
	closed    chan bool
	immediate chan bool
	host      string
	size      int
	tasks     map[string]*GroupedTask
	waiting   map[string]*LeviGroupedTask
	wg        *sync.WaitGroup
}

func NewLevi(conn *Connection, size int) *Levi {
	return &Levi{
		conn:      conn,
		inTask:    make(chan *Task),
		closed:    make(chan bool),
		immediate: make(chan bool),
		host:      conn.host,
		size:      size,
		tasks:     make(map[string]*GroupedTask),
		waiting:   make(map[string]*LeviGroupedTask),
		wg:        &sync.WaitGroup{},
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
			key := fmt.Sprintf("%s:%s:%s", task.Name, task.Uid, task.Version)
			if _, exists := self.tasks[key]; !exists {
				self.tasks[key] = &GroupedTask{
					Name:    task.Name,
					Uid:     task.Uid,
					Id:      uuid.New(),
					Version: task.Version,
					Tasks:   []*Task{},
				}
			}
			self.tasks[key].Tasks = append(self.tasks[key].Tasks, task)
			if self.Len() >= self.size {
				logger.Debug("send tasks when full")
				self.SendTasks()
			}
		case <-self.closed:
			if self.Len() != 0 {
				logger.Debug("send tasks before close")
				self.SendTasks()
			}
			finish = true
		case <-self.immediate:
			if self.Len() > 0 {
				logger.Debug("send tasks immediate")
				self.SendTasks()
			}
		case <-time.After(time.Second * time.Duration(config.Task.Dispatch)):
			if self.Len() != 0 {
				logger.Debug("send tasks when timeout")
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
	close(self.immediate)
	self.wg.Wait()
	self.conn.CloseConnection()
}

func (self *Levi) SendTasks() {
	self.wg.Add(len(self.tasks))
	for _, groupedTask := range self.tasks {
		go func(groupedTask *GroupedTask) {
			defer self.wg.Done()
			leviGroupedTask := groupedTask.ToLeviGroupedTask()
			self.waiting[groupedTask.Id] = leviGroupedTask
			if err := self.conn.ws.WriteJSON(&leviGroupedTask); err != nil {
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
		default:

			taskUUID := taskReply.Id

			if taskUUID == "__STATUS__" || len(taskReply.Status) != 0 {
				logger.Info("special commands")
				UpdateContainerStatus(host, taskReply.Status)
				continue
			}

			if taskReply.Test != nil {
				logger.Info("test result ", taskReply.Test)
			}

			// 普通任务的返回值
			lgt, exists := self.waiting[taskUUID]
			if !exists {
				logger.Info("not exists, ignore")
				continue
			}

			app := GetApplicationByNameAndVersion(lgt.Name, lgt.Version)
			if app == nil {
				logger.Info("app 没了")
				continue
			}

			lt := lgt.Tasks
			doAdd(app, host, lt.Add, taskReply.Add)
			doBuild(app, host, lt.Build, taskReply.Build)
			doRemove(lt.Remove, taskReply.Remove)

			if lgt.NeedToRestartNginx() {
				hub.done <- app.Id
			}

			if len(GetContainerByHostAndApp(host, app)) == 0 {
				hub.immediate <- true
			}

			delete(self.waiting, taskUUID)
		}
	}
}

func (self *Levi) Len() int {
	count := 0
	for _, value := range self.tasks {
		count += len(value.Tasks)
	}
	return count
}

func UpdateContainerStatus(host *Host, statuses []*StatusInfo) {
	for _, status := range statuses {
		if status.Type == "die" {
			logger.Info("Should delete ", status.Id, " of ", status.Appname)
			if c := GetContainerByCid(status.Id); c != nil {
				hub.Dispatch(host.IP, RemoveContainerTask(c))
			} else {
				logger.Info("Container ", status.Id, " already removed")
			}
		}
	}
}

func doAdd(app *Application, host *Host, tasks []*AddTask, replies []string) {
	for i := 0; i < len(tasks); i = i + 1 {
		task, retval := tasks[i], replies[i]

		logger.Debug("add tasks[i]: ", task)
		logger.Debug("add taskReplies[i]: ", retval)

		if task == nil || retval == "" {
			logger.Info("task/retval is nil, ignore")
			continue
		}

		if task.IsTest() {
			// 暂时先不存这个
			logger.Info("task tests, ignore")
			continue
		}

		NewContainer(app, host, task.Bind, retval, task.Daemon)
	}
}

func doBuild(app *Application, host *Host, tasks []*BuildTask, replies []string) {
	for i := 0; i < len(tasks); i = i + 1 {
		task, retval := tasks[i], replies[i]

		logger.Debug("build tasks[i]: ", task)
		logger.Debug("build taskReplies[i]: ", retval)

		if task == nil || retval == "" {
			logger.Info("task/retval is nil, ignore")
			continue
		}

		appUserUid := app.UserUid()
		staticPath := path.Join(config.Nginx.Staticdir, app.Name, app.Version)
		staticSrcPath := path.Join(config.Nginx.Staticsrcdir, app.Name, app.Version)
		if err := CopyFiles(staticPath, staticSrcPath, appUserUid, appUserUid); err != nil {
			logger.Info("copy files error: ", err)
		}
	}
}

func doRemove(tasks []*RemoveTask, replies []bool) {
	for i := 0; i < len(tasks); i = i + 1 {
		task, retval := tasks[i], replies[i]

		logger.Debug("remove tasks[i]: ", task)
		logger.Debug("remove taskReplies[i]: ", retval)

		if task == nil || !retval {
			logger.Info("task/retval is nil, ignore")
			continue
		}

		if old := GetContainerByCid(task.Container); old != nil {
			old.Delete()
		} else {
			logger.Info("要删的容器已经不在了")
		}
	}
}
