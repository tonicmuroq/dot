package main

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"./config"
	"./models"
	. "./utils"

	"code.google.com/p/go-uuid/uuid"
)

type Levi struct {
	conn      *Connection
	inTask    chan *models.Task
	immediate chan bool
	host      string
	size      int
	tasks     map[string]*models.GroupedTask
	waiting   map[string]*models.LeviGroupedTask
	running   bool
	wg        *sync.WaitGroup
}

func NewLevi(conn *Connection, size int) *Levi {
	return &Levi{
		conn:      conn,
		inTask:    make(chan *models.Task),
		immediate: make(chan bool),
		host:      conn.host,
		size:      size,
		tasks:     make(map[string]*models.GroupedTask),
		waiting:   make(map[string]*models.LeviGroupedTask),
		running:   true,
		wg:        &sync.WaitGroup{},
	}
}

func (self *Levi) Host() *models.Host {
	return models.GetHostByIP(self.host)
}

func (self *Levi) WaitTask() {
	defer self.wg.Done()
	var task *models.Task
	for self.running {
		select {
		case task, self.running = <-self.inTask:
			Logger.Debug("levi got task ", task, self.running)
			if task == nil {
				// 有nil, 无视掉
				break
			}
			key := fmt.Sprintf("%s:%s:%s", task.Name, task.Uid, task.Version)
			if _, exists := self.tasks[key]; !exists {
				self.tasks[key] = &models.GroupedTask{
					Name:    task.Name,
					Uid:     task.Uid,
					ID:      uuid.New(),
					Version: task.Version,
					Tasks:   []*models.Task{},
				}
			}
			self.tasks[key].Tasks = append(self.tasks[key].Tasks, task)
			if self.Len() >= self.size {
				Logger.Debug("send tasks when full")
				self.SendTasks()
			}
		case <-self.immediate:
			if self.Len() > 0 {
				Logger.Debug("send tasks immediate")
				self.SendTasks()
			}
		case <-time.After(time.Second * time.Duration(config.Config.Task.Dispatch)):
			if self.Len() != 0 {
				Logger.Debug("send tasks when timeout")
				self.SendTasks()
			}
		}
	}
}

func (self *Levi) Close() {
	self.wg.Add(1)
	self.running = false
	self.inTask <- nil
	close(self.inTask)
	close(self.immediate)
	self.wg.Wait()
	self.conn.CloseConnection()
}

func (self *Levi) SendTasks() {
	self.wg.Add(len(self.tasks))
	for _, groupedTask := range self.tasks {
		go func(groupedTask *models.GroupedTask) {
			defer self.wg.Done()
			leviGroupedTask := groupedTask.ToLeviGroupedTask()
			self.waiting[groupedTask.ID] = leviGroupedTask
			if err := self.conn.ws.WriteJSON(&leviGroupedTask); err != nil {
				Logger.Info(err, "JSON write error")
			}
		}(groupedTask)
	}
	self.wg.Wait()
	self.tasks = make(map[string]*models.GroupedTask)
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
		var taskReply models.TaskReply
		switch err := self.conn.ws.ReadJSON(&taskReply); {
		case err != nil:
			Logger.Info("read json error: ", err)
			finish = true
		default:

			taskUUID := taskReply.ID

			if taskUUID == "__STATUS__" {
				doStatus(host, taskReply.Data)
				continue
			}

			lgt, exists := self.waiting[taskUUID]
			if !exists {
				Logger.Info(taskUUID, " not exists, ignore")
				continue
			}

			av := models.GetVersion(lgt.Name, lgt.Version)
			if av == nil {
				Logger.Info(fmt.Sprintf("AppVersion %v", av), "没了")
				continue
			}

			lt := lgt.Tasks

			switch taskReply.Type {
			case models.ADD:
				doAdd(av, host, lt.Add, taskReply)
			case models.REMOVE:
				doRemove(lt.Remove, taskReply)
			case models.BUILD:
				doBuild(av, lt.Build, taskReply)
			case models.TEST:
				doTest(av, lt.Add, taskReply)
			case models.INFO:
				doStatus(host, taskReply.Data)
			}

			if lgt.Done() {

				if lgt.NeedToRestartNginx() {
					hub.done <- av.ID
				}

				if len(models.GetContainerByHostAndApp(host, av.Name)) == 0 {
					hub.immediate <- true
				}

				delete(self.waiting, taskUUID)
			}
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

// status没有关联task, 不要担心
func doStatus(host *models.Host, data string) {
	r := strings.Split(data, "|")
	// status|name|containerId
	if len(r) != 3 {
		return
	}
	status, name, containerId := r[0], r[1], r[2]
	if status == "die" {
		Logger.Info("Should delete ", containerId, " of ", name)
		if c := models.GetContainerByCid(containerId); c != nil {
			// 不要发了
			// hub.Dispatch(host.IP, models.RemoveContainerTask(c))
		} else {
			Logger.Info("Container ", containerId, " already removed")
		}
	}
}

func doAdd(av *models.AppVersion, host *models.Host, tasks []*models.AddTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data
	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}
	if job := models.GetJob(task.ID); job != nil {
		switch reply.Done {
		case true:
			if !task.IsTest() {
				if retval != "" {
					job.Done(models.SUCC, retval)
					models.NewContainer(av, host, task.Bind, retval, task.Daemon, task.SubApp)
				} else {
					job.Done(models.FAIL, retval)
				}
			} else {
				// 理论上不可能出现任务是测试Type是ADD_TASK同时又是Done为true的
				job.Done(models.FAIL, retval)
			}
			task.Done()
		case false:
			if !task.IsTest() {
				Logger.Debug("Add output stream: ", retval)
				// TODO 记录下AddContainer的日志流返回
			} else {
				// 如果测试任务就没返回容器值, 那么直接挂
				if retval != "" {
					job.SetResult(retval)
					models.NewContainer(av, host, task.Bind, retval, task.Test, task.SubApp)
				} else {
					job.Done(models.FAIL, "failed when create testing container")
				}
			}
		}
	}

}

func doTest(av *models.AppVersion, tasks []*models.AddTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data
	b := streamLogHub.GetBufferedLog(task.ID, true)
	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}
	if job := models.GetJob(task.ID); job != nil {
		b.Feed(retval)
		switch reply.Done {
		case false:
			// TODO 记录下TestContainer的日志流返回
			Logger.Debug("Test output stream: ", retval)
		case true:
			if task.IsTest() {
				container := models.GetContainerByCid(job.Result)
				if container == nil {
					return
				}
				if retval == "0" {
					job.Done(models.SUCC, fmt.Sprintf("%s|%s", container.IdentID, retval))
				} else {
					job.Done(models.FAIL, fmt.Sprintf("%s|%s", container.IdentID, retval))
				}
				container.Delete()
				streamLogHub.RemoveBufferedLog(task.ID)
			}
			task.Done()
		}
	}

}

func doBuild(av *models.AppVersion, tasks []*models.BuildTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data
	b := streamLogHub.GetBufferedLog(task.ID, true)
	b.Feed(retval)

	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}

	switch reply.Done {
	case false:
		Logger.Debug("Build output stream: ", retval)
	case true:
		appUserUid := av.UserUID()
		staticPath := path.Join(config.Config.Nginx.Staticdir, av.Name, av.Version)
		staticSrcPath := path.Join(config.Config.Nginx.Staticsrcdir, av.Name, av.Version)
		if err := CopyFiles(staticPath, staticSrcPath, appUserUid, appUserUid); err != nil {
			Logger.Info("copy files error: ", err)
		}
		if job := models.GetJob(task.ID); job != nil {
			if retval != "" {
				job.Done(models.SUCC, retval)
				av.SetImageAddr(retval)
			} else {
				job.Done(models.FAIL, retval)
			}
		}
		streamLogHub.RemoveBufferedLog(task.ID)
		task.Done()
	}
}

func doRemove(tasks []*models.RemoveTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data

	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}
	switch reply.Done {
	case false:
		Logger.Debug("Remove output stream: ", retval)
	case true:
		if old := models.GetContainerByCid(task.Container); old != nil {
			old.Delete()
		} else {
			Logger.Info("要删的容器已经不在了")
		}
		// build 根据返回值来判断是不是成功
		if job := models.GetJob(task.ID); job != nil {
			if retval == "1" {
				job.Done(models.SUCC, "removed")
			} else {
				job.Done(models.FAIL, "not removed")
			}
		}
		task.Done()
	}
}
