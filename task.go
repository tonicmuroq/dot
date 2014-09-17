package main

import (
	"strings"
)

const (
	AddContainer    = 1
	RemoveContainer = 2
	UpdateContainer = 3
	BuildImage      = 4
	TestApplication = 5
)

type BuildTask struct {
	Name    string
	Version string
	Group   string
	Base    string
	Build   string
	Static  string
	Schema  string
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
	Test      string
	Container string
	Build     BuildTask
}

type GroupedTask struct {
	Name  string
	Uid   int
	Id    string
	Type  int
	Tasks []*Task
}

type TaskReply map[string][]interface{}

// Task
func AddContainerTask(app *Application, host *Host, daemon bool) *Task {
	var bind int
	var daemonId string
	if daemon {
		bind = 0
		daemonId = CreateRandomHexString(app.Name, 7)
	} else {
		bind = GetPortFromHost(host)
		daemonId = ""
		// 没有可以用的端口了
		logger.Debug("task bind: ", bind)
		if bind == 0 {
			return nil
		}
	}

	appYaml, err := app.GetAppYaml()
	if err != nil {
		logger.Debug("app.yaml error: ", err)
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
		Host:    host.IP,
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
		Host:      host.IP,
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
		daemonId = CreateRandomHexString(app.Name, 7)
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
		Host:      host.IP,
		Type:      UpdateContainer,
		Uid:       app.UserUid(),
		Bind:      bind,
		Memory:    config.Task.Memory,
		Cpus:      config.Task.Cpus,
		Daemon:    daemonId,
		Container: container.ContainerId}
	return &task
}

// build任务的name就是应用的projectname
// build任务的version就是应用的version, 都是7位的git版本号
// build任务的build就是应用的build
// 也就是告诉dot, 我需要用base来构建group下的app应用
func BuildImageTask(app *Application, group, base string) *Task {
	if app == nil {
		return nil
	}
	appYaml, err := app.GetAppYaml()
	if err != nil {
		logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Build) == 0 {
		logger.Debug("build task error: need build in app.yaml")
		return nil
	}
	buildTask := BuildTask{
		Name:    app.Pname,
		Version: app.Version,
		Group:   group,
		Base:    base,
		Build:   appYaml.Build[0],
		Static:  appYaml.Static,
		Schema:  "", // 先来个空的吧
	}
	task := Task{
		Name:    app.Name,
		Uid:     app.UserUid(),
		Type:    BuildImage,
		Build:   buildTask,
		Version: app.Version,
	}
	return &task
}

// test task
func TestApplicationTask(app *Application, host *Host) *Task {
	var bind int
	testId := CreateRandomHexString(app.Name, 7)

	appYaml, err := app.GetAppYaml()
	if err != nil {
		logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Test) == 0 {
		logger.Debug("test task error: need test in app.yaml")
		return nil
	}
	testCmdString := appYaml.Test[0]
	testCmd := strings.Split(testCmdString, " ")
	port := appYaml.Port

	task := Task{
		Name:    strings.ToLower(app.Name),
		Version: app.Version,
		Port:    port,
		Cmd:     testCmd,
		Host:    host.IP,
		Type:    TestApplication,
		Uid:     app.UserUid(),
		Bind:    bind,
		Memory:  config.Task.Memory,
		Cpus:    config.Task.Cpus,
		Test:    testId}
	return &task
}
