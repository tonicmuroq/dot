package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"path"
	"sync"
	"time"
)

type Levi struct {
	conn      *Connection
	inTask    chan *Task
	closed    chan bool
	immediate chan bool
	host      string
	size      int
	tasks     map[string]*GroupedTask
	waiting   map[string]*GroupedTask
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
		waiting:   make(map[string]*GroupedTask),
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
			key := fmt.Sprintf("%s:%s:%s:%s", task.Name, task.Uid, task.Type, task.Version)
			if _, exists := self.tasks[key]; !exists {
				self.tasks[key] = &GroupedTask{
					Name:    task.Name,
					Uid:     task.Uid,
					Type:    task.Type,
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

				// 如果 taskUUID 是一个特殊命令
				// 那么根据特殊命令做出回应, 不再执行下面的步骤
				if taskUUID == "__status__" {
					logger.Info("special commands")
					// TODO 执行特殊命令
					continue
				}

				// 普通任务的返回值
				groupedTask, exists := self.waiting[taskUUID]
				if !exists {
					logger.Info("not exists, ignore")
					continue
				}

				// 如果这个任务是获取容器信息, 那么根据返回值来更新容器状态
				// 由于返回值的数量会比任务数量要多, 因此直接执行
				// 不再执行下面的步骤
				if groupedTask.Type == HostInfo {
					logger.Info("update container status base on result")
					// TODO 更新容器状态
					continue
				}

				// 普通的任务和返回值数量对等的任务
				// 包括 AddContainer, RemoveContainer, UpdateContainer,
				// BuildImage, TestApplication
				if len(groupedTask.Tasks) != len(taskReplies) {
					logger.Info("task reply is not zippable with tasks, ignore")
					continue
				}
				app := GetApplicationByNameAndVersion(groupedTask.Name, groupedTask.Version)
				if app == nil {
					logger.Info("app 没了")
					continue
				}

				for i := 0; i < len(groupedTask.Tasks); i = i + 1 {
					task, retval := groupedTask.Tasks[i], taskReplies[i]

					logger.Debug("tasks[i]: ", task)
					logger.Debug("taskReplies[i]: ", retval)

					if task == nil || retval == nil {
						logger.Info("task/retval is nil, ignore")
						continue
					}

					logger.Debug("Task Feedback", task.Type)

					switch task.Type {
					case AddContainer:
						NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
					case RemoveContainer:
						if old := GetContainerByCid(task.Container); old != nil {
							old.Delete()
						} else {
							logger.Info("要删的容器已经不在了")
						}
					case UpdateContainer:
						if old := GetContainerByCid(task.Container); old != nil {
							old.Delete()
						}
						NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
					case TestApplication:
						// 测试的第一次返回是用于测试的容器id, ignore
						if v, ok := retval.(string); ok {
							logger.Debug("testing container id is: ", v)
							cleanWaiting = false
							// 测试的第二次返回是测试的结果
						} else if _, ok := retval.(map[string]interface{}); ok {
							// TODO 记录测试结果
						}
					}
				}

				if NeedToRestartNginx(groupedTask.Type) {
					hub.done <- app.Id
				}

				switch groupedTask.Type {
				case RemoveContainer:
					if len(GetContainerByHostAndApp(host, app)) == 0 {
						hub.immediate <- true
					}
				case BuildImage:
					appUserUid := app.UserUid()
					staticPath := path.Join(config.Nginx.Staticdir, app.Name, app.Version)
					staticSrcPath := path.Join(config.Nginx.Staticsrcdir, app.Name, app.Version)
					if err := CopyFiles(staticPath, staticSrcPath, appUserUid, appUserUid); err != nil {
						logger.Info("copy files error: ", err)
					}
				}

				// 测试第一次返回并不清空任务
				// TODO 一个变量只给测试用太浪费
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
