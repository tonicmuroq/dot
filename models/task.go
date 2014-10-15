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

	SUCC = 0
	FAIL = 1
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
}

type RemoveTask struct {
	Id        int
	Name      string
	Version   string
	Container string
	RmImage   bool
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

type TestResult struct {
	ExitCode int
	Err      string
}

type StatusInfo struct {
	Type    string
	Appname string
	Id      string
}

type TaskReply struct {
	Id     string
	Build  []string
	Add    []string
	Remove []bool
	Test   map[string]*TestResult
	Status []*StatusInfo
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
		RmImage:   false,
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

func AddContainerTask(app *Application, host *Host) *Task {

	appYaml, err := app.GetAppYaml()
	if err != nil {
		Logger.Debug("app.yaml error: ", err)
		return nil
	}

	var bind int
	var daemonId string

	if appYaml.Daemon {
		bind = 0
		daemonId = CreateRandomHexString(app.Name, 7)
	} else {
		bind = GetPortFromHost(host)
		daemonId = ""
		// 没有可以用的端口了
		if bind == 0 {
			return nil
		}
	}

	cmdString := appYaml.Cmd[0]
	cmd := strings.Split(cmdString, " ")
	port := appYaml.Port

	st := NewStoredTask(app.Id, AddContainer)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}

	task := Task{
		Id:       st.Id,
		Name:     strings.ToLower(app.Name),
		Version:  app.Version,
		Port:     port,
		Cmd:      cmd,
		Host:     host.IP,
		Type:     AddContainer,
		Uid:      app.UserUid(),
		Bind:     bind,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		Daemon:   daemonId}
	return &task
}

func RemoveContainerTask(container *Container) *Task {
	app := container.Application()
	host := container.Host()
	if app == nil || host == nil {
		return nil
	}

	st := NewStoredTask(app.Id, RemoveContainer)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}

	task := Task{
		Id:        st.Id,
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

	st := NewStoredTask(app.Id, UpdateContainer)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}

	task := Task{
		Id:        st.Id,
		Name:      strings.ToLower(app.Name),
		Version:   app.Version,
		Port:      port,
		Cmd:       cmd,
		Host:      host.IP,
		Type:      UpdateContainer,
		Uid:       app.UserUid(),
		Bind:      bind,
		Memory:    config.Config.Task.Memory,
		CpuShare:  config.Config.Task.CpuShare,
		CpuSet:    config.Config.Task.CpuSet,
		Daemon:    daemonId,
		Container: container.ContainerId}
	return &task
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
	}
	task := Task{
		Id:      st.Id,
		Name:    strings.ToLower(app.Name),
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
		Logger.Debug("app.yaml error: ", err)
		return nil
	}
	if len(appYaml.Test) == 0 {
		Logger.Debug("test task error: need test in app.yaml")
		return nil
	}
	testCmdString := appYaml.Test[0]
	testCmd := strings.Split(testCmdString, " ")
	port := appYaml.Port

	st := NewStoredTask(app.Id, TestApplication)
	if st == nil {
		Logger.Info("task not inserted")
		return nil
	}
	task := Task{
		Id:       st.Id,
		Name:     strings.ToLower(app.Name),
		Version:  app.Version,
		Port:     port,
		Cmd:      testCmd,
		Host:     host.IP,
		Type:     TestApplication,
		Uid:      app.UserUid(),
		Bind:     bind,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		Test:     testId}
	return &task
}

// host info task
func HostInfoTask(host *Host) *Task {
	if host == nil {
		return nil
	}
	return &Task{
		Name:    "__host_info__",
		Version: "__info_version__",
		Host:    host.IP,
		Type:    HostInfo,
		Uid:     0,
	}
}
