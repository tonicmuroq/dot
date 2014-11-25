package models

import (
	"../config"
	. "../utils"
	"strings"
	"time"
)

const (
	AddContainer    = 1
	RemoveContainer = 2
	UpdateContainer = 3
	BuildImage      = 4
	TestApplication = 5
	HostInfo        = 6

	RUNNING = 0
	DONE    = 1

	// 原本的语义已经改了...
	// 这个应该叫做 YES/NO
	SUCC = 1
	FAIL = 0

	ADD_TASK    = 1
	REMOVE_TASK = 2
	BUILD_TASK  = 3
	INFO_TASK   = 4
	TEST_TASK   = 5
)

type BuildTask struct {
	Id      int
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
	Id        int
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
	done      bool
}

type RemoveTask struct {
	Id        int
	Name      string
	Version   string
	Container string
	RmImage   bool
	done      bool
}

type Task struct {
	Id        int
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
	Build     BuildTask
	RmImage   bool
}

type GroupedTask struct {
	Name    string
	Uid     int
	Id      string
	Version string
	Tasks   []*Task
}

type LeviTasks struct {
	Build  []*BuildTask
	Add    []*AddTask
	Remove []*RemoveTask
}

type LeviGroupedTask struct {
	Id      string
	Uid     int
	Name    string
	Version string
	Info    bool
	Tasks   *LeviTasks
}

type StatusInfo struct {
	Type    string
	Appname string
	Id      string
}

// sent back from Levi
type TaskReply struct {
	Id    string
	Done  bool
	Index int
	Type  int
	Data  string
}

type StoredTask struct {
	Id       int
	AppId    int // 对应应用
	Status   int // 对应状态, Running/Done
	Succ     int // 成功/失败
	Kind     int // 类型, Add/Remove/Update/Build/Test
	Result   string
	Created  time.Time `orm:"auto_now_add;type(datetime)"`
	Finished time.Time `orm:"auto_now;type(datetime)"`
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
		Id:      self.Id,
		Uid:     self.Uid,
		Name:    self.Name,
		Version: self.Version,
		Info:    false,
	}
	lt := &LeviTasks{}
	for _, task := range self.Tasks {
		switch task.Type {
		case AddContainer, TestApplication:
			lt.Add = append(lt.Add, task.ToAddTask())
		case RemoveContainer:
			lt.Remove = append(lt.Remove, task.ToRemoveTask())
		case UpdateContainer:
			lt.Add = append(lt.Add, task.ToAddTask())
			lt.Remove = append(lt.Remove, task.ToRemoveTask())
		case BuildImage:
			lt.Build = append(lt.Build, task.ToBuildTask())
		case HostInfo:
			lgt.Info = true
		}
	}
	lgt.Tasks = lt
	return lgt
}

// StoredTask
func (self *StoredTask) TableIndex() [][]string {
	return [][]string{
		[]string{"AppId"},
		[]string{"Status"},
		[]string{"Kind"},
	}
}

func GetStoredTaskById(id int) *StoredTask {
	var st StoredTask
	if err := db.QueryTable(new(StoredTask)).Filter("Id", id).One(&st); err != nil {
		return nil
	} else {
		return &st
	}
}

func NewStoredTask(appId, kind int) *StoredTask {
	st := StoredTask{AppId: appId, Status: RUNNING, Succ: FAIL, Kind: kind}
	if _, err := db.Insert(&st); err == nil {
		return &st
	} else {
		return nil
	}
}

func GetStoredTaskByAppAndRet(appId int, ret string) *StoredTask {
	var st StoredTask
	if err := db.QueryTable(new(StoredTask)).Filter("AppId", appId).Filter("Result", ret).One(&st); err != nil {
		return nil
	} else {
		return &st
	}
}

func (self *StoredTask) Done(succ int, result string) {
	self.Status = DONE
	self.Succ = succ
	self.Result = result
	db.Update(self)
}

func (self *StoredTask) SetResult(result string) {
	self.Result = result
	db.Update(self)
}

// Task
func (self *Task) ToAddTask() *AddTask {
	return &AddTask{
		Id:        self.Id,
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
		Id:        self.Id,
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

func AddContainerTask(app *Application, host *Host, daemon bool) *Task {
	appYaml, err := app.GetAppYaml()
	if err != nil {
		Logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Daemon) == 0 && daemon {
		Logger.Debug("no daemon defined in app.yaml")
		return nil
	}
	if len(appYaml.Cmd) == 0 && !daemon {
		Logger.Debug("no cmd defined in app.yaml")
		return nil
	}

	bind := 0
	daemonId := ""
	cmd := []string{}

	if daemon {
		bind = 0
		daemonId = CreateRandomHexString(app.Name, 7)
		cmd = strings.Split(appYaml.Daemon[0], " ")
	} else {
		bind = GetPortFromHost(host)
		if bind == 0 {
			return nil
		}
		daemonId = ""
		cmd = strings.Split(appYaml.Cmd[0], " ")
	}

	st := NewStoredTask(app.Id, AddContainer)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}

	return &Task{
		Id:       st.Id,
		Name:     strings.ToLower(app.Name),
		Version:  app.Version,
		Port:     appYaml.Port,
		Cmd:      cmd,
		Host:     host.IP,
		Type:     AddContainer,
		Uid:      app.UserUid(),
		Bind:     bind,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		Daemon:   daemonId,
	}
}

func RemoveContainerTask(container *Container) *Task {
	app := container.Application()
	host := container.Host()
	if host == nil || app == nil {
		return nil
	}
	rmImg := false
	if cs := GetContainerByHostAndApp(host, app); len(cs) == 1 {
		rmImg = true
	}
	if app == nil || host == nil {
		return nil
	}

	st := NewStoredTask(app.Id, RemoveContainer)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}

	return &Task{
		Id:        st.Id,
		Name:      strings.ToLower(app.Name),
		Version:   app.Version,
		Host:      host.IP,
		Type:      RemoveContainer,
		Uid:       0,
		Container: container.ContainerId,
		RmImage:   rmImg,
	}
}

func UpdateContainerTask(container *Container, app *Application) *Task {
	host := container.Host()
	oldApp := container.Application()
	if host == nil || oldApp == nil {
		return nil
	}
	appYaml, err := app.GetAppYaml()
	if err != nil {
		return nil
	}
	rmImg := false
	if cs := GetContainerByHostAndApp(host, oldApp); len(cs) == 1 {
		rmImg = true
	}

	bind := 0
	daemonId := ""
	cmd := []string{}

	if container.IdentId != "" {
		bind = 0
		daemonId = CreateRandomHexString(app.Name, 7)
		cmd = strings.Split(appYaml.Daemon[0], " ")
	} else {
		bind = GetPortFromHost(host)
		if bind == 0 {
			return nil
		}
		daemonId = ""
		cmd = strings.Split(appYaml.Cmd[0], " ")
	}

	st := NewStoredTask(app.Id, UpdateContainer)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}

	return &Task{
		Id:        st.Id,
		Name:      strings.ToLower(app.Name),
		Version:   app.Version,
		Port:      appYaml.Port,
		Cmd:       cmd,
		Host:      host.IP,
		Type:      UpdateContainer,
		Uid:       app.UserUid(),
		Bind:      bind,
		Memory:    config.Config.Task.Memory,
		CpuShare:  config.Config.Task.CpuShare,
		CpuSet:    config.Config.Task.CpuSet,
		Daemon:    daemonId,
		Container: container.ContainerId,
		RmImage:   rmImg,
	}
}

// build任务的name就是应用的projectname
// build任务的version就是应用的version, 都是7位的git版本号
// build任务的build就是应用的build
// 也就是告诉dot, 我需要用base来构建group下的app应用
func BuildImageTask(app *Application, base string) *Task {
	if app == nil {
		return nil
	}
	appYaml, err := app.GetAppYaml()
	if err != nil {
		Logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Build) == 0 {
		Logger.Debug("build task error: need build in app.yaml")
		return nil
	}
	st := NewStoredTask(app.Id, BuildImage)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}
	buildTask := BuildTask{
		Id:      st.Id,
		Name:    app.Pname,
		Version: app.Version,
		Group:   app.Group,
		Base:    base,
		Build:   appYaml.Build[0],
		Static:  appYaml.Static,
		Schema:  "", // 先来个空的吧
		done:    false,
	}
	return &Task{
		Id:      st.Id,
		Name:    strings.ToLower(app.Name),
		Uid:     app.UserUid(),
		Type:    BuildImage,
		Build:   buildTask,
		Version: app.Version,
	}
}

// test task
func TestApplicationTask(app *Application, host *Host) *Task {
	testId := CreateRandomHexString(app.Name, 7)

	appYaml, err := app.GetAppYaml()
	if err != nil {
		Logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Test) == 0 {
		Logger.Debug("test task error: need test in app.yaml")
		return nil
	}
	testCmd := strings.Split(appYaml.Test[0], " ")

	st := NewStoredTask(app.Id, TestApplication)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}
	return &Task{
		Id:       st.Id,
		Name:     strings.ToLower(app.Name),
		Version:  app.Version,
		Port:     appYaml.Port,
		Cmd:      testCmd,
		Host:     host.IP,
		Type:     TestApplication,
		Uid:      app.UserUid(),
		Bind:     0,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		Test:     testId}
}
