package main

import (
	"code.google.com/p/go-uuid/uuid"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	AddContainer    = 1
	RemoveContainer = 2
	UpdateContainer = 3
	BuildImage      = 4
	Memory          = 1024 * 1024 * 1024
	Cpus            = 100
)

type TaskHub struct {
	queue chan *[]byte
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

var taskhub *TaskHub

// TaskHub
func (self *TaskHub) GetTask() *Task {
	return <-self.queue
}

func (self *TaskHub) AddTask(task *Task) {
	self.queue <- task
	log.Println(len(self.queue))
	if len(self.queue) >= self.size {
		log.Println("full, do dispatch")
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
			log.Println("period check")
			self.Dispatch()
			log.Println("period check done")
		} else {
			log.Println("empty")
		}
		time.Sleep(30 * time.Second)
	}
}

func (self *TaskHub) Dispatch() {
	self.mutex.Lock()
	count := len(self.queue)
	for i := 0; i < count; i = i + 1 {
		d := self.GetTask()
		self.wg.Add(1)
		log.Println(string(*d))
	}
	self.wg.Wait()
	self.mutex.Unlock()
	log.Println("finish, restart nginx")
}

func init() {
	// TODO size shall be in arguments
	taskhub = &TaskHub{queue: make(chan *Task, 5), size: 5, wg: &sync.WaitGroup{}, mutex: &sync.Mutex{}}
}

// Task
func AddContainerTask(app *Application, host *Host, daemon bool) *Task {
	var port int
	var daemonId string
	if daemon {
		port = 0
		daemonId = uuid.New()
	} else {
		port = GetPortFromHost(host)
		daemonId = ""
		// 没有可以用的端口了
		if port == 0 {
			return nil
		}
	}

	appYaml := app.GetAppYaml()
	cmdString := appYaml.Cmd[0]
	cmd := strings.Split(cmdString, " ")

	task := Task{
		Name:    strings.ToLower(app.Name),
		Version: app.Version,
		Port:    app.Port,
		Cmd:     cmd,
		Host:    host.Ip,
		Type:    AddContainer,
		Uid:     app.UserUid(),
		Bind:    port,
		Memory:  Memory,
		Cpus:    Cpus,
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

	var port int
	var daemonId string
	if container.DaemonId != "" {
		port = 0
		daemonId = uuid.New()
	} else {
		port = GetPortFromHost(host)
		// 不够端口玩了
		if port == 0 {
			return nil
		}
		daemonId = ""
	}

	appYaml := app.GetAppYaml()
	cmdString := appYaml.Cmd[0]
	cmd := strings.Split(cmdString, " ")

	task := Task{
		Name:      strings.ToLower(app.Name),
		Version:   app.Version,
		Port:      app.Port,
		Cmd:       cmd,
		Host:      host.Ip,
		Type:      UpdateContainer,
		Uid:       app.UserUid(),
		Bind:      port,
		Memory:    Memory,
		Cpus:      Cpus,
		Daemon:    daemonId,
		Container: container.ContainerId}
	return &task
}
