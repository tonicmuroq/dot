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
					Id:      uuid.New(),
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
			self.waiting[groupedTask.Id] = leviGroupedTask
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

			taskUUID := taskReply.Id

			if taskUUID == "__STATUS__" {
				doStatus(host, taskReply.Data)
				continue
			}

			lgt, exists := self.waiting[taskUUID]
			if !exists {
				Logger.Info(taskUUID, " not exists, ignore")
				continue
			}

			app := models.GetApplicationByNameAndVersion(lgt.Name, lgt.Version)
			if app == nil {
				Logger.Info(fmt.Sprintf("App %v", app), "没了")
				continue
			}

			lt := lgt.Tasks

			switch taskReply.Type {
			case models.ADD_TASK:
				doAdd(app, host, lt.Add, taskReply)
			case models.REMOVE_TASK:
				doRemove(lt.Remove, taskReply)
			case models.BUILD_TASK:
				doBuild(app, lt.Build, taskReply)
			case models.TEST_TASK:
				doTest(app, lt.Add, taskReply)
			case models.INFO_TASK:
				doStatus(host, taskReply.Data)
			}

			if lgt.Done() {

				if lgt.NeedToRestartNginx() {
					hub.done <- app.Id
				}

				if len(models.GetContainerByHostAndApp(host, app)) == 0 {
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

func doAdd(app *models.Application, host *models.Host, tasks []*models.AddTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data
	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}
	if st := models.GetStoredTaskById(task.Id); st != nil {
		switch reply.Done {
		case true:
			if !task.IsTest() {
				st.Done(models.SUCC, retval)
				models.NewContainer(app, host, task.Bind, retval, task.Daemon)
			} else {
				// 理论上不可能出现任务是测试Type是ADD_TASK同时又是Done为true的
				st.Done(models.FAIL, retval)
			}
			tasks[reply.Index] = nil
		case false:
			if !task.IsTest() {
				Logger.Debug("Add output stream: ", retval)
				// TODO 记录下AddContainer的日志流返回
			} else {
				// 如果测试任务就没返回容器值, 那么直接挂
				if retval != "" {
					st.SetResult(retval)
					models.NewContainer(app, host, task.Bind, retval, task.Test)
				} else {
					st.Done(models.FAIL, "failed when create testing container")
				}
			}
		}
	}

}

func doTest(app *models.Application, tasks []*models.AddTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data
	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}
	if st := models.GetStoredTaskById(task.Id); st != nil {
		switch reply.Done {
		case false:
			// TODO 记录下TestContainer的日志流返回
			Logger.Debug("Test output stream: ", retval)
		case true:
			if task.IsTest() {
				Logger.Info("Test result")
				containerId := st.Result
				Logger.Debug("test result ", retval)
				Logger.Debug("test container id ", containerId)
				container := models.GetContainerByCid(containerId)
				Logger.Debug("test container ", container)
				if container == nil {
					return
				}
				if retval == "0" {
					st.Done(models.SUCC, fmt.Sprintf("%s|%s", container.IdentId, retval))
				} else {
					st.Done(models.FAIL, fmt.Sprintf("%s|%s", container.IdentId, retval))
				}
				container.Delete()
			}
			tasks[reply.Index] = nil
		}
	}

}

func doBuild(app *models.Application, tasks []*models.BuildTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data

	Logger.Debug("build tasks[i]: ", task)
	Logger.Debug("build taskReplies[i]: ", retval)

	if task == nil {
		Logger.Info("task/retval is nil, ignore")
		return
	}

	switch reply.Done {
	case false:
		Logger.Debug("Build output stream: ", retval)
	case true:
		appUserUid := app.UserUid()
		staticPath := path.Join(config.Config.Nginx.Staticdir, app.Name, app.Version)
		staticSrcPath := path.Join(config.Config.Nginx.Staticsrcdir, app.Name, app.Version)
		if err := CopyFiles(staticPath, staticSrcPath, appUserUid, appUserUid); err != nil {
			Logger.Info("copy files error: ", err)
		}
		if st := models.GetStoredTaskById(task.Id); st != nil {
			if retval != "" {
				st.Done(models.SUCC, retval)
				app.SetImageAddr(retval)
			} else {
				st.Done(models.FAIL, retval)
			}
		}
		tasks[reply.Index] = nil
	}
}

func doRemove(tasks []*models.RemoveTask, reply models.TaskReply) {
	task, retval := tasks[reply.Index], reply.Data

	Logger.Debug("remove tasks[i]: ", task)
	Logger.Debug("remove taskReplies[i]: ", retval)

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
		if st := models.GetStoredTaskById(task.Id); st != nil {
			if retval == "1" {
				st.Done(models.SUCC, "removed")
			} else {
				st.Done(models.FAIL, "not removed")
			}
		}
		tasks[reply.Index] = nil
	}
}
