package main

import (
	"code.google.com/p/go-uuid/uuid"
	"strings"
	"sync"
	"time"
)

const (
	AddContainer    = 1
	RemoveContainer = 2
	UpdateContainer = 3
	BuildImage      = 4
)

type TaskHub struct {
	queue chan *Task
	size  int
	wg    *sync.WaitGroup
	mutex *sync.Mutex
}

type Task struct {
	Name      string
	Version   string
	Port      int
	Cmd       []string
	Host      string
	Type      int
	Uid       int
	Bind      int
	Memory    int
	Cpus      int
	Daemon    string
	Container string
}

var taskHub *TaskHub

type GroupedTask struct {
	Name  string
	Uid   int
	Id    string
	Type  int
	Tasks []Task
}

// TaskHub
func (self *TaskHub) GetTask() *Task {
	return <-self.queue
}

func (self *TaskHub) AddTask(task *Task) {
	self.queue <- task
	logger.Debug(len(self.queue))
	if len(self.queue) >= self.size {
		logger.Debug("full, do dispatch")
		self.Dispatch()
	}
}

func (self *TaskHub) FinishOneTask() {
	self.wg.Done()
}

func (self *TaskHub) Run() {
	for {
		count := len(self.queue)
		if count > 0 {
			logger.Debug("period check")
			self.Dispatch()
			logger.Debug("period check done")
		} else {
			logger.Debug("empty")
		}
		time.Sleep(config.Task.DispatchInterval * time.Second)
	}
}

func (self *TaskHub) Dispatch() {
	self.mutex.Lock()
	count := len(self.queue)
	self.wg.Add(count)
	for i := 0; i < count; i = i + 1 {
		_ = self.GetTask()
	}
	self.wg.Wait()
	self.mutex.Unlock()
	logger.Debug("finish, restart nginx")
}

func (self *TaskHub) RegroupTasks() *GroupedTask {
	return nil
}

func init() {
	taskHub = &TaskHub{
		queue: make(chan *Task, config.Task.QueueSize),
		size:  config.Task.QueueSize,
		wg:    &sync.WaitGroup{},
		mutex: &sync.Mutex{},
	}
}

// Task
func AddContainerTask(app *Application, host *Host, daemon bool) *Task {
	var bind int
	var daemonId string
	if daemon {
		bind = 0
		daemonId = uuid.New()
	} else {
		bind = GetPortFromHost(host)
		daemonId = ""
		// 没有可以用的端口了
		if bind == 0 {
			return nil
		}
	}

	appYaml, err := app.GetAppYaml()
	if err != nil {
		return nil
	}
	cmdString := appYaml.Cmd[0]
	cmd := strings.Split(cmdString, " ")
	port := appYaml.Port

	task := Task{
		Name:    strings.ToLower(app.Name),
		Version: app.Version,
		Port:    port,
		Cmd:     cmd,
		Host:    host.Ip,
		Type:    AddContainer,
		Uid:     app.UserUid(),
		Bind:    bind,
		Memory:  config.Task.Memory,
		Cpus:    config.Task.Cpus,
		Daemon:  daemonId}
	return &task
}

func RemoveContainerTask(container *Container) *Task {
	app := container.Application()
	host := container.Host()
	if app == nil || host == nil {
		return nil
	}
	task := Task{
		Name:      strings.ToLower(app.Name),
		Version:   app.Version,
		Host:      host.Ip,
		Type:      RemoveContainer,
		Uid:       0,
		Container: container.ContainerId}
	return &task
}

func UpdateContainerTask(container *Container, app *Application) *Task {
	host := container.Host()
	if host == nil {
		return nil
	}

	var bind int
	var daemonId string
	if container.DaemonId != "" {
		bind = 0
		daemonId = uuid.New()
	} else {
		bind = GetPortFromHost(host)
		// 不够端口玩了
		if bind == 0 {
			return nil
		}
		daemonId = ""
	}

	appYaml, err := app.GetAppYaml()
	if err != nil {
		return nil
	}
	cmdString := appYaml.Cmd[0]
	cmd := strings.Split(cmdString, " ")
	port := appYaml.Port

	task := Task{
		Name:      strings.ToLower(app.Name),
		Version:   app.Version,
		Port:      port,
		Cmd:       cmd,
		Host:      host.Ip,
		Type:      UpdateContainer,
		Uid:       app.UserUid(),
		Bind:      bind,
		Memory:    config.Task.Memory,
		Cpus:      config.Task.Cpus,
		Daemon:    daemonId,
		Container: container.ContainerId}
	return &task
}
