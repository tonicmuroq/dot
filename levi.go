package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"net"
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
	waiting map[string][]*Task
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
		waiting: make(map[string][]*Task),
		wg:      &sync.WaitGroup{},
	}
}

func (self *Levi) WaitTask() {
	defer self.wg.Done()
	finish := false
	for !finish {
		select {
		case task := <-self.inTask:
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
			logger.Debug("time check ", self.Len())
			if self.Len() != 0 {
				logger.Debug("send tasks")
				self.SendTasks()
			}
		}
	}
}

func (self *Levi) Close() {
	self.wg.Add(1)
	self.closed <- true
	self.wg.Wait()
	self.conn.CloseConnection()
}

func (self *Levi) SendTasks() {
	logger.Debug(self.tasks)
	self.wg.Add(len(self.tasks))
	for _, groupedTask := range self.tasks {
		go func(groupedTask *GroupedTask) {
			defer self.wg.Done()
			self.waiting[groupedTask.Id] = groupedTask.Tasks
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
	defer func() {
		self.Close()
		hub.RemoveLevi(self.host)
	}()
	for !self.conn.closed {
		var taskReply TaskReply
		switch err := self.conn.ws.ReadJSON(&taskReply); {
		case err != nil:
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				logger.Info(err)
				self.conn.CloseConnection()
			}
		case err == nil:
			logger.Debug("taskReply", taskReply)
			for taskUUID, taskReplies := range taskReply {
				tasks, exists := self.waiting[taskUUID]
				if !exists || (exists && len(tasks) != len(taskReplies)) {
					continue
				}
				for i := 0; i < len(tasks); i = i + 1 {
					task := tasks[i]
					retval := taskReplies[i]
					switch task.Type {
					case AddContainer:
						app := GetApplicationByNameAndVersion(task.Name, task.Version)
						host := GetHostByIp(task.Host)
						if app == nil || host == nil {
							logger.Info("app/host 没了")
							continue
						}
						NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
					case RemoveContainer:
						old := GetContainerByCid(task.Container)
						if old == nil {
							logger.Info("要删的容器已经不在了")
							continue
						}
						old.Delete()
					case UpdateContainer:
						old := GetContainerByCid(task.Container)
						if old != nil {
							old.Delete()
						}
						app := GetApplicationByNameAndVersion(task.Name, task.Version)
						host := GetHostByIp(task.Host)
						if app == nil || host == nil {
							logger.Info("app/host 没了")
							continue
						}
						NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
					}
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
