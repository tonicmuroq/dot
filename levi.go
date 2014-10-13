package main

import (
	"./config"
	"./models"
	. "./utils"
	"fmt"
	"path"
	"sync"
	"time"

	"code.google.com/p/go-uuid/uuid"
)

type Levi struct {
	conn      *Connection
	inTask    chan *models.Task
	closed    chan bool
	immediate chan bool
	host      string
	size      int
	tasks     map[string]*models.GroupedTask
	waiting   map[string]*models.LeviGroupedTask
	wg        *sync.WaitGroup
}

func NewLevi(conn *Connection, size int) *Levi {
	return &Levi{
		conn:      conn,
		inTask:    make(chan *models.Task),
		closed:    make(chan bool),
		immediate: make(chan bool),
		host:      conn.host,
		size:      size,
		tasks:     make(map[string]*models.GroupedTask),
		waiting:   make(map[string]*models.LeviGroupedTask),
		wg:        &sync.WaitGroup{},
	}
}

func (self *Levi) Host() *models.Host {
	return models.GetHostByIP(self.host)
}

func (self *Levi) WaitTask() {
	defer self.wg.Done()
	finish := false
	for !finish {
		select {
		case task, ok := <-self.inTask:
			Logger.Debug("levi got task ", task, ok)
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
		case <-self.closed:
			if self.Len() != 0 {
				Logger.Debug("send tasks before close")
				self.SendTasks()
			}
			finish = true
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

			if taskUUID == "__STATUS__" || len(taskReply.Status) != 0 {
				Logger.Info("special commands")
				UpdateContainerStatus(host, taskReply.Status)
				continue
			}

			if taskReply.Test != nil {
				Logger.Info("test result ", taskReply.Test)
			}

			// 普通任务的返回值
			lgt, exists := self.waiting[taskUUID]
			if !exists {
				Logger.Info("not exists, ignore")
				continue
			}

			app := models.GetApplicationByNameAndVersion(lgt.Name, lgt.Version)
			if app == nil {
				Logger.Info("app 没了")
				continue
			}

			lt := lgt.Tasks
			doAdd(app, host, lt.Add, taskReply.Add)
			doBuild(app, host, lt.Build, taskReply.Build)
			doRemove(lt.Remove, taskReply.Remove)

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

func (self *Levi) Len() int {
	count := 0
	for _, value := range self.tasks {
		count += len(value.Tasks)
	}
	return count
}

func UpdateContainerStatus(host *models.Host, statuses []*models.StatusInfo) {
	for _, status := range statuses {
		if status.Type == "die" {
			Logger.Info("Should delete ", status.Id, " of ", status.Appname)
			if c := models.GetContainerByCid(status.Id); c != nil {
				hub.Dispatch(host.IP, models.RemoveContainerTask(c))
			} else {
				Logger.Info("Container ", status.Id, " already removed")
			}
		}
	}
}

func doAdd(app *models.Application, host *models.Host, tasks []*models.AddTask, replies []string) {
	for i := 0; i < len(tasks); i = i + 1 {
		task, retval := tasks[i], replies[i]

		Logger.Debug("add tasks[i]: ", task)
		Logger.Debug("add taskReplies[i]: ", retval)

		if task == nil || retval == "" {
			Logger.Info("task/retval is nil, ignore")
			continue
		}

		if task.IsTest() {
			// 暂时先不存这个
			Logger.Info("task tests, ignore")
			continue
		}

		models.NewContainer(app, host, task.Bind, retval, task.Daemon)
	}
}

func doBuild(app *models.Application, host *models.Host, tasks []*models.BuildTask, replies []string) {
	for i := 0; i < len(tasks); i = i + 1 {
		task, retval := tasks[i], replies[i]

		Logger.Debug("build tasks[i]: ", task)
		Logger.Debug("build taskReplies[i]: ", retval)

		if task == nil || retval == "" {
			Logger.Info("task/retval is nil, ignore")
			continue
		}

		appUserUid := app.UserUid()
		staticPath := path.Join(config.Config.Nginx.Staticdir, app.Name, app.Version)
		staticSrcPath := path.Join(config.Config.Nginx.Staticsrcdir, app.Name, app.Version)
		if err := CopyFiles(staticPath, staticSrcPath, appUserUid, appUserUid); err != nil {
			Logger.Info("copy files error: ", err)
		}
	}
}

func doRemove(tasks []*models.RemoveTask, replies []bool) {
	for i := 0; i < len(tasks); i = i + 1 {
		task, retval := tasks[i], replies[i]

		Logger.Debug("remove tasks[i]: ", task)
		Logger.Debug("remove taskReplies[i]: ", retval)

		if task == nil || !retval {
			Logger.Info("task/retval is nil, ignore")
			continue
		}

		if old := models.GetContainerByCid(task.Container); old != nil {
			old.Delete()
		} else {
			Logger.Info("要删的容器已经不在了")
		}
	}
}
