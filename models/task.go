package models

import (
	"../config"
	. "../utils"
	"strings"
)

const (
	ADDCONTAINER    = 1
	REMOVECONTAINER = 2
	UPDATECONTAINER = 3
	BUILDIMAGE      = 4
	TESTAPPLICATION = 5
)

type BuildTask struct {
	ID      int `json:"id"`
	Name    string
	Version string
	Group   string
	Base    string
	Build   string
	Static  string
	Schema  string
	done    bool
}

type AddTask struct {
	ID        int `json:"id"`
	Name      string
	Version   string
	Bind      int
	Port      int
	Cmd       []string
	Memory    int
	CpuShares int
	CpuSet    string
	Daemon    string
	Test      string
	SubApp    string `json:sub_app`
	done      bool
}

type RemoveTask struct {
	ID        int `json:"id"`
	Name      string
	Version   string
	Container string
	RmImage   bool
	SubApp    string `json:sub_app`
	done      bool
}

type Task struct {
	ID        int `json:"id"`
	Name      string
	Version   string
	Port      int
	Cmd       []string
	Host      string
	Type      int
	Uid       int
	Bind      int
	Memory    int
	CpuShare  int
	CpuSet    string
	Daemon    string
	Test      string
	Container string
	SubApp    string `json:sub_app`
	Build     BuildTask
	RmImage   bool
}

type GroupedTask struct {
	Name    string
	Uid     int
	ID      string `json: "id"`
	Version string
	Tasks   []*Task
}

type LeviTasks struct {
	Build  []*BuildTask
	Add    []*AddTask
	Remove []*RemoveTask
}

type LeviGroupedTask struct {
	ID      string `json:"id"`
	Uid     int
	Name    string
	Version string
	Info    bool
	Tasks   *LeviTasks
}

// sent back from Levi
type TaskReply struct {
	ID    string `json:"id"`
	Done  bool
	Index int
	Type  int
	Data  string
}

func (self *BuildTask) Done() {
	self.done = true
}

func (self *AddTask) Done() {
	self.done = true
}

func (self *RemoveTask) Done() {
	self.done = true
}

func (self *LeviTasks) Done() bool {
	sumLength := len(self.Build) + len(self.Add) + len(self.Remove)
	if sumLength == 0 {
		// 本身就是空的
		return false
	}
	for _, build := range self.Build {
		if build != nil && !build.done {
			return false
		}
	}
	for _, add := range self.Add {
		if add != nil && !add.done {
			return false
		}
	}
	for _, remove := range self.Remove {
		if remove != nil && !remove.done {
			return false
		}
	}
	return true
}

func (self *GroupedTask) ToLeviGroupedTask() *LeviGroupedTask {
	lgt := &LeviGroupedTask{
		ID:      self.ID,
		Uid:     self.Uid,
		Name:    self.Name,
		Version: self.Version,
		Info:    false,
	}
	lt := &LeviTasks{}
	for _, task := range self.Tasks {
		switch task.Type {
		case ADDCONTAINER, TESTAPPLICATION:
			lt.Add = append(lt.Add, task.ToAddTask())
		case REMOVECONTAINER:
			lt.Remove = append(lt.Remove, task.ToRemoveTask())
		case UPDATECONTAINER:
			lt.Add = append(lt.Add, task.ToAddTask())
			lt.Remove = append(lt.Remove, task.ToRemoveTask())
		case BUILDIMAGE:
			lt.Build = append(lt.Build, task.ToBuildTask())
		}
	}
	lgt.Tasks = lt
	return lgt
}

// Task
func (self *Task) ToAddTask() *AddTask {
	return &AddTask{
		ID:        self.ID,
		Name:      self.Name,
		Version:   self.Version,
		Bind:      self.Bind,
		Port:      self.Port,
		Cmd:       self.Cmd,
		Memory:    self.Memory,
		CpuShares: self.CpuShare,
		CpuSet:    self.CpuSet,
		Daemon:    self.Daemon,
		Test:      self.Test,
		done:      false,
	}
}

func (self *Task) ToBuildTask() *BuildTask {
	return &self.Build
}

func (self *Task) ToRemoveTask() *RemoveTask {
	return &RemoveTask{
		ID:        self.ID,
		Name:      self.Name,
		Version:   self.Version,
		Container: self.Container,
		RmImage:   self.RmImage,
		done:      false,
	}
}

// AddTask
func (self *AddTask) IsTest() bool {
	return self.Test != ""
}

// LeviGroupedTask
// only add/remove needs to retart nginx
// and test shall be ignored
func (self *LeviGroupedTask) NeedToRestartNginx() bool {
	lt := self.Tasks
	// Test not counted
	addCount := 0
	for _, add := range lt.Add {
		if !add.IsTest() {
			addCount += 1
		}
	}
	return addCount > 0 || len(lt.Remove) != 0
}

func (self *LeviGroupedTask) Done() bool {
	return self.Tasks != nil && self.Tasks.Done()
}

func AddContainerTask(av *AppVersion, host *Host, appYaml *AppYaml, daemon bool) *Task {
	if len(appYaml.Daemon) == 0 && daemon {
		Logger.Info("no daemon defined in app.yaml")
		return nil
	}
	if len(appYaml.Cmd) == 0 && !daemon {
		Logger.Info("no cmd defined in app.yaml")
		return nil
	}

	bind := 0
	daemonID := ""
	cmd := []string{}
	subapp := ""

	if appYaml.Appname != av.Name {
		subapp = appYaml.Appname
	}

	if daemon {
		bind = 0
		daemonID = CreateRandomHexString(av.Name, 7)
		cmd = strings.Split(appYaml.Daemon[0], " ")
	} else {
		bind = GetPortFromHost(host)
		if bind == 0 {
			return nil
		}
		daemonID = ""
		cmd = strings.Split(appYaml.Cmd[0], " ")
	}

	job := NewJob(av, ADDCONTAINER)
	if job == nil {
		Logger.Info("task not inserted")
		return nil
	}

	return &Task{
		ID:       job.ID,
		Name:     strings.ToLower(av.Name),
		Version:  av.Version,
		Port:     appYaml.Port,
		Cmd:      cmd,
		Host:     host.IP,
		Type:     ADDCONTAINER,
		Uid:      av.UserUID(),
		Bind:     bind,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		Daemon:   daemonID,
		SubApp:   subapp,
	}
}

func RemoveContainerTask(container *Container) *Task {
	av := container.AppVersion()
	host := container.Host()
	if host == nil || av == nil {
		return nil
	}

	rmImg := false
	if cs := GetContainerByHostAndAppVersion(host, av); len(cs) == 1 {
		rmImg = true
	}

	job := NewJob(av, REMOVECONTAINER)
	if job == nil {
		Logger.Info("task not inserted")
		return nil
	}

	return &Task{
		ID:        job.ID,
		Name:      strings.ToLower(av.Name),
		Version:   av.Version,
		Host:      host.IP,
		Type:      REMOVECONTAINER,
		Uid:       0,
		Container: container.ContainerID,
		RmImage:   rmImg,
		SubApp:    container.SubApp,
	}
}

func UpdateContainerTask(container *Container, av *AppVersion) *Task {
	host := container.Host()
	oav := container.AppVersion()
	if host == nil || oav == nil {
		return nil
	}
	appYaml, err := av.GetSubAppYaml(container.SubApp)
	if err != nil {
		return nil
	}
	rmImg := false
	if cs := GetContainerByHostAndAppVersion(host, oav); len(cs) == 1 {
		rmImg = true
	}

	bind := 0
	daemonID := ""
	cmd := []string{}

	if container.IdentID != "" {
		bind = 0
		daemonID = CreateRandomHexString(av.Name, 7)
		cmd = strings.Split(appYaml.Daemon[0], " ")
	} else {
		bind = GetPortFromHost(host)
		if bind == 0 {
			return nil
		}
		daemonID = ""
		cmd = strings.Split(appYaml.Cmd[0], " ")
	}

	job := NewJob(av, UPDATECONTAINER)
	if job == nil {
		Logger.Info("task not inserted")
		return nil
	}

	return &Task{
		ID:        job.ID,
		Name:      strings.ToLower(av.Name),
		Version:   av.Version,
		Port:      appYaml.Port,
		Cmd:       cmd,
		Host:      host.IP,
		Type:      UPDATECONTAINER,
		Uid:       av.UserUID(),
		Bind:      bind,
		Memory:    config.Config.Task.Memory,
		CpuShare:  config.Config.Task.CpuShare,
		CpuSet:    config.Config.Task.CpuSet,
		Daemon:    daemonID,
		Container: container.ContainerID,
		SubApp:    container.SubApp,
		RmImage:   rmImg,
	}
}

// build任务的name就是应用的projectname
// build任务的version就是应用的version, 都是7位的git版本号
// build任务的build就是应用的build
// 也就是告诉dot, 我需要用base来构建group下的app应用
func BuildImageTask(av *AppVersion, base string) *Task {
	if av == nil {
		return nil
	}
	app := GetApplication(av.Name)
	if app == nil {
		return nil
	}
	appYaml, err := av.GetAppYaml()
	if err != nil {
		Logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Build) == 0 {
		Logger.Debug("build task error: need build in app.yaml")
		return nil
	}
	job := NewJob(av, BUILDIMAGE)
	if job == nil {
		Logger.Info("task not inserted")
		return nil
	}
	buildTask := BuildTask{
		ID:      job.ID,
		Name:    app.Pname,
		Version: av.Version,
		Group:   app.Namespace,
		Base:    base,
		Build:   appYaml.Build[0],
		Static:  appYaml.Static,
		Schema:  "", // 先来个空的吧
		done:    false,
	}
	return &Task{
		ID:      job.ID,
		Name:    strings.ToLower(av.Name),
		Uid:     av.UserUID(),
		Type:    BUILDIMAGE,
		Build:   buildTask,
		Version: av.Version,
	}
}

// test task
func TestApplicationTask(av *AppVersion, host *Host) *Task {
	testID := CreateRandomHexString(av.Name, 7)

	appYaml, err := av.GetAppYaml()
	if err != nil {
		Logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Test) == 0 {
		Logger.Debug("test task error: need test in app.yaml")
		return nil
	}
	testCmd := strings.Split(appYaml.Test[0], " ")

	job := NewJob(av, TESTAPPLICATION)
	if job == nil {
		Logger.Info("task not inserted")
		return nil
	}
	return &Task{
		ID:       job.ID,
		Name:     strings.ToLower(av.Name),
		Version:  av.Version,
		Port:     appYaml.Port,
		Cmd:      testCmd,
		Host:     host.IP,
		Type:     TESTAPPLICATION,
		Uid:      av.UserUID(),
		Bind:     0,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		SubApp:   "",
		Test:     testID,
	}
}
