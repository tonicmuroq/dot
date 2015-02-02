package types

import (
	"strings"

	"code.google.com/p/go-uuid/uuid"

	"config"
	. "utils"
)

const (
	ADDCONTAINER    = 1
	REMOVECONTAINER = 2
	UPDATECONTAINER = 3
	BUILDIMAGE      = 4
	TESTAPPLICATION = 5
)

type Task struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	SubApp  string `json:"_"`
	Type    int    `json:"-"`
	done    bool   `json:"-"`

	// run options
	Cmd      []string `json:"cmd,omitempty"`
	Uid      int      `json:"uid,omitempty"`
	Bind     int      `json:"bind,omitempty"`
	Port     int      `json:"port,omitempty"`
	Memory   int      `json:"memory,omitempty"`
	CpuShare int      `json:"cpushare,omitempty"`
	CpuSet   string   `json:"cpuset,omitempty"`
	Daemon   string   `json:"daemon,omitempty"`

	// remove options
	Container string `json:"container,omitempty"`
	RmImage   bool   `json:"rmimg,omitempty"`

	// test options
	Test string `json:"test,omitempty"`

	// build options
	Group  string `json:"group,omitempty"`
	Build  string `json:"build,omitempty"`
	Base   string `json:"base,omitempty"`
	Static string `json:"static,omitempty"`
	Schema string `json:"schema,omitempty"`
}

type LeviTasks struct {
	Build  []*Task `json:"build"`
	Add    []*Task `json:"add"`
	Remove []*Task `json:"remove"`
}

type LeviGroupedTask struct {
	UUID    string     `json:"id"`
	Name    string     `json:"name"`
	Uid     int        `json:"uid"`
	Version string     `json:"version"`
	Tasks   *LeviTasks `json:"tasks"`
}

// sent back from Levi
type TaskReply struct {
	ID    string `json:"id"`
	Done  bool   `json:"done"`
	Index int    `json:"index"`
	Type  int    `json:"type"`
	Data  string `json:"data"`
}

func NewLeviGroupedTask(name string, uid int, version string) *LeviGroupedTask {
	leviTasks := &LeviTasks{
		Build:  []*Task{},
		Add:    []*Task{},
		Remove: []*Task{},
	}
	return &LeviGroupedTask{
		Name:    name,
		Uid:     uid,
		UUID:    uuid.New(),
		Version: version,
		Tasks:   leviTasks,
	}
}

func (lt *LeviTasks) Done() bool {
	if len(lt.Build)+len(lt.Add)+len(lt.Remove) == 0 {
		return false
	}
	for _, build := range lt.Build {
		if build != nil && !build.done {
			return false
		}
	}
	for _, add := range lt.Add {
		if add != nil && !add.done {
			return false
		}
	}
	for _, remove := range lt.Remove {
		if remove != nil && !remove.done {
			return false
		}
	}
	return true
}

func (lgt *LeviGroupedTask) AppendTask(task *Task) {
	switch task.Type {
	case ADDCONTAINER, TESTAPPLICATION:
		lgt.Tasks.Add = append(lgt.Tasks.Add, task)
	case REMOVECONTAINER:
		lgt.Tasks.Remove = append(lgt.Tasks.Remove, task)
	case UPDATECONTAINER:
		lgt.Tasks.Add = append(lgt.Tasks.Add, task)
		lgt.Tasks.Remove = append(lgt.Tasks.Remove, task)
	case BUILDIMAGE:
		lgt.Tasks.Build = append(lgt.Tasks.Build, task)
	}
}

// 一批任务里, 如果一些SubApp有增减, 那么这些SubApp对应的配置需要重启
// 测试任务不算
func (lgt *LeviGroupedTask) RestartSubAppNames() []string {
	g := map[string]int{}
	for _, add := range lgt.Tasks.Add {
		if !add.IsTest() {
			g[add.SubApp] += 1
		}
	}
	for _, remove := range lgt.Tasks.Remove {
		g[remove.SubApp] += 1
	}
	r := make([]string, len(g))
	i := 0
	for key, _ := range g {
		r[i] = key
		i = i + 1
	}
	return r
}

// 一个任务组里, 如果这个levi所属的host上这个sub app的容器数为0
// 那么说明删完了, 需要马上重启
func (lgt *LeviGroupedTask) RestartImmediately(host *Host, name string) bool {
	cg := map[string]int{}
	for _, c := range GetContainerByHostAndApp(host, name) {
		cg[c.SubApp] += 1
	}
	for _, remove := range lgt.Tasks.Remove {
		if _, e := cg[remove.SubApp]; !e {
			return true
		}
	}
	return false
}

func (lgt *LeviGroupedTask) Done() bool {
	return lgt.Tasks != nil && lgt.Tasks.Done()
}

func (lgt *LeviGroupedTask) Len() int {
	return len(lgt.Tasks.Add) + len(lgt.Tasks.Remove) + len(lgt.Tasks.Build)
}

func (t *Task) IsTest() bool {
	return t.Test != ""
}

func (t *Task) Done() {
	t.done = true
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
		daemonID = RandomString(7)
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
		daemonID = RandomString(7)
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
	return &Task{
		ID:      job.ID,
		Name:    strings.ToLower(av.Name),
		Uid:     av.UserUID(),
		Type:    BUILDIMAGE,
		Version: av.Version,
		Group:   app.Namespace,
		Base:    base,
		Build:   appYaml.Build[0],
		Static:  appYaml.Static,
		Schema:  "", // 先来个空的吧
		done:    false,
	}
}

func TestApplicationTask(av *AppVersion, host *Host) *Task {
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
		Type:     TESTAPPLICATION,
		Uid:      av.UserUID(),
		Bind:     0,
		Memory:   config.Config.Task.Memory,
		CpuShare: config.Config.Task.CpuShare,
		CpuSet:   config.Config.Task.CpuSet,
		SubApp:   "",
		Test:     RandomString(7),
	}
}
