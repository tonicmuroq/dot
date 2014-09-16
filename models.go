package main

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	"github.com/coreos/go-etcd/etcd"
	_ "github.com/go-sql-driver/mysql"
	"path"
	"sync"
)

const (
	appPathPrefix = "/NBE/"
)

var db orm.Ormer
var etcdClient *etcd.Client
var portMutex sync.Mutex

type Host struct {
	Id   int
	IP   string `orm:"column(ip)"`
	Name string
}

type HostPort struct {
	Id     int
	HostId int
	Port   int
}

type Container struct {
	Id          int
	Port        int
	ContainerId string
	DaemonId    string
	HostId      int
	AppId       int
}

type User struct {
	Id   int
	Name string
}

type Application struct {
	Id      int
	Name    string
	Version string
	Pname   string
	User    *User `orm:"rel(fk)"`
}

type AppYaml struct {
	Appname  string   `json:appname`
	Runtime  string   `json:runtime`
	Port     int      `json:port`
	Cmd      []string `json:cmd`
	Test     []string `json:test`
	Build    []string `json:build`
	Services []string `json:services`
	Static   string   `json:static`
}

type ConfigYaml map[string]interface{}

// models 初始化
// 连接 mysql
// 连接 etcd
// 可能还需要连接 redis
func init() {
	// mysql
	orm.RegisterDataBase(config.Db.Name, config.Db.Use, config.Db.Url, 30)
	orm.RegisterModel(new(Application), new(User), new(Host), new(Container), new(HostPort))
	orm.RunSyncdb(config.Db.Name, false, false)
	db = orm.NewOrm()

	// etcd
	etcdClient = etcd.NewClient(config.Etcd.Machines)
	if config.Etcd.Sync {
		etcdClient.SyncCluster()
	}

	// Mutex
	portMutex = sync.Mutex{}
}

// Application
func (self *Application) TableUnique() [][]string {
	return [][]string{
		[]string{"Name", "Version"},
	}
}

func GetApplicationById(appId int) *Application {
	var app Application
	err := db.QueryTable(new(Application)).Filter("Id", appId).One(&app)
	if err != nil {
		return nil
	}
	return &app
}

func NewApplication(projectname, version, appyaml, configyaml string) *Application {
	// 调整yaml
	if configyaml == "" {
		configyaml = "{}"
	}
	var appYamlJson AppYaml
	var oconfigYamlJson ConfigYaml
	var copyConfigYamlJson = make(ConfigYaml)
	var testConfigYamlJson = make(ConfigYaml)

	if err1, err2 := JSONDecode(appyaml, &appYamlJson), JSONDecode(configyaml, &oconfigYamlJson); err1 != nil || err2 != nil {
		logger.Debug("app.yaml error: ", err1)
		logger.Debug("config.yaml error: ", err2)
		return nil
	}

	for k, v := range oconfigYamlJson {
		copyConfigYamlJson[k] = v
		testConfigYamlJson[k] = v
	}

	// 生成新用户
	appName := appYamlJson.Appname
	user := User{Name: appName}
	if _, id, err := db.ReadOrCreate(&user, "Name"); err == nil {
		user.Id = int(id)
	} else {
		logger.Info("create user failed", err)
		return nil
	}

	// 用户绑定应用
	app := Application{Name: appName, Version: version, Pname: projectname, User: &user}
	if _, err := db.Insert(&app); err != nil {
		return nil
	}

	// 配置文件更改
	for _, service := range appYamlJson.Services {
		if service == "mysql" {
			if dbInfoString := app.GetOrCreateDbInfo(); dbInfoString != "" {
				var d map[string]interface{}
				if err := JSONDecode(dbInfoString, &d); err == nil {
					oconfigYamlJson["mysql"] = d
					// 没得可以分配的, 先写这个吧, 一定会挂
					testConfigYamlJson["mysql"] = d
				} else {
					logger.Info("mysql create failed")
				}
			}
		}
		if service == "redis" {
			d := CreateRedis(&app)
			oconfigYamlJson["redis"] = d
			// 同上
			testConfigYamlJson["mysql"] = d
		}
	}

	if newConfigYaml, err := YAMLEncode(oconfigYamlJson); err == nil {
		etcdClient.Create((&app).GetYamlPath("config"), newConfigYaml, 0)
	}
	if appYaml, err := YAMLEncode(appYamlJson); err == nil {
		etcdClient.Create((&app).GetYamlPath("app"), appYaml, 0)
	}
	if configYaml, err := YAMLEncode(copyConfigYamlJson); err == nil {
		if len(configYaml) == 0 {
			etcdClient.Create((&app).GetYamlPath("original-config"), "", 0)
		} else {
			etcdClient.Create((&app).GetYamlPath("original-config"), configYaml, 0)
		}
	}
	if configYaml, err := YAMLEncode(testConfigYamlJson); err == nil {
		if len(configYaml) == 0 {
			etcdClient.Create((&app).GetYamlPath("test"), "", 0)
		} else {
			etcdClient.Create((&app).GetYamlPath("test"), configYaml, 0)
		}
	}

	// 生成必须路径
	etcdClient.CreateDir(path.Join(appPathPrefix, "_Apps", app.Name, "daemons"), 0)
	etcdClient.CreateDir(path.Join(appPathPrefix, "_Apps", app.Name, "apps"), 0)

	return &app
}

func GetApplicationByNameAndVersion(name, version string) *Application {
	var app Application
	err := db.QueryTable(new(Application)).Filter("Name", name).Filter("Version", version).RelatedSel().One(&app)
	if err != nil {
		return nil
	}
	return &app
}

func (self *Application) GetOrCreateDbInfo() string {
	cpath := path.Join(appPathPrefix, self.Name, "mysql")
	if _, err := etcdClient.Create(cpath, "", 0); err == nil {
		db, err := CreateDatabase(self)
		if err != nil {
			return ""
		}
		if json, err := JSONEncode(db); err == nil {
			etcdClient.Set(cpath, json, 0)
			return json
		}
		return ""
	} else {
		if r, err := etcdClient.Get(cpath, false, false); err == nil {
			return r.Node.Value
		}
		return ""
	}
}

func (self *Application) GetYamlPath(cpath string) string {
	return path.Join(appPathPrefix, self.Name, self.Version, fmt.Sprintf("%s.yaml", cpath))
}

func (self *Application) GetAppYaml() (*AppYaml, error) {
	var appYaml AppYaml
	cpath := self.GetYamlPath("app")
	r, err := etcdClient.Get(cpath, false, false)
	if err != nil {
		return &appYaml, err
	}
	if r.Node.Dir {
		return &appYaml, errors.New("should not be dir")
	}
	if err = YAMLDecode(r.Node.Value, &appYaml); err != nil {
		return &appYaml, err
	}
	return &appYaml, nil
}

func (self *Application) GetConfigYaml() (*ConfigYaml, error) {
	var configYaml ConfigYaml
	cpath := self.GetYamlPath("config")
	r, err := etcdClient.Get(cpath, false, false)
	if err != nil {
		return &configYaml, err
	}
	if r.Node.Dir {
		return &configYaml, errors.New("should not be dir")
	}
	if err = YAMLDecode(r.Node.Value, &configYaml); err != nil {
		return &configYaml, err
	}
	return &configYaml, nil
}

func (self *Application) UserUid() int {
	return self.User.Id
}

func (self *Application) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("AppId", self.Id).OrderBy("Port").All(&cs)
	return cs
}

func (self *Application) Hosts() []*Host {
	var rs orm.ParamsList
	var hosts []*Host
	_, err := db.Raw("SELECT distinct(host_id) FROM container WHERE app_id=?", self.Id).ValuesFlat(&rs)
	if err == nil && len(rs) > 0 {
		db.QueryTable(new(Host)).Filter("id__in", rs).All(&hosts)
	}
	return hosts
}

// User
func (self *User) TableUnique() [][]string {
	return [][]string{
		[]string{"Name"},
	}
}

// Host
func (self *Host) TableUnique() [][]string {
	return [][]string{
		[]string{"IP"},
	}
}

func NewHost(ip, name string) *Host {
	host := Host{IP: ip, Name: name}
	if _, id, err := db.ReadOrCreate(&host, "IP"); err == nil {
		host.Id = int(id)
		return &host
	}
	return nil
}

func GetHostById(hostId int) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("Id", hostId).One(&host)
	if err != nil {
		return nil
	}
	return &host
}

func GetHostByIP(ip string) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("IP", ip).One(&host)
	if err != nil {
		return nil
	}
	return &host
}

// 注意里面可能有nil
func GetHostsByIPs(ips []string) []*Host {
	hosts := make([]*Host, len(ips))
	for i, ip := range ips {
		hosts[i] = GetHostByIP(ip)
	}
	return hosts
}

func (self *Host) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostId", self.Id).OrderBy("Port").All(&cs)
	return cs
}

func (self *Host) Ports() []int {
	var ports []*HostPort
	db.QueryTable(new(HostPort)).Filter("HostId", self.Id).OrderBy("Port").All(&ports)
	r := make([]int, len(ports))
	for i := 0; i < len(ports); i = i + 1 {
		r[i] = ports[i].Port
	}
	return r
}

func (self *Host) AddPort(port int) {
	hostPort := HostPort{HostId: self.Id, Port: port}
	db.Insert(&hostPort)
}

func (self *Host) RemovePort(port int) {
	db.Raw("DELETE FROM host_port WHERE host_id=? AND port=?", self.Id, port).Exec()
}

// Container
func (self *Container) TableIndex() [][]string {
	return [][]string{
		[]string{"AppId"},
		[]string{"ContainerId"},
		[]string{"host_id"}, /* TODO 有点tricky */
	}
}

func (self *Container) Application() *Application {
	return GetApplicationById(self.AppId)
}

func (self *Container) Host() *Host {
	return GetHostById(self.HostId)
}

func (self *Container) Delete() bool {
	host := self.Host()
	if host != nil {
		host.RemovePort(self.Port)
	} else {
		logger.Debug("Host not found when deleting container")
		return false
	}
	if _, err := db.Delete(&Container{Id: self.Id}); err == nil {
		return true
	}
	return false
}

func NewContainer(app *Application, host *Host, port int, containerId, daemonId string) *Container {
	c := Container{Port: port, ContainerId: containerId, DaemonId: daemonId, AppId: app.Id, HostId: host.Id}
	if _, err := db.Insert(&c); err == nil {
		return &c
	}
	return nil
}

func GetContainerByCid(cid string) *Container {
	var container Container
	err := db.QueryTable(new(Container)).Filter("ContainerId", cid).One(&container)
	if err != nil {
		return nil
	}
	return &container
}

func GetContainerByHostAndApp(host *Host, app *Application) []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostId", host.Id).Filter("AppId", app.Id).OrderBy("Port").All(&cs)
	return cs
}

// 获取一个host上的可用的一个端口
// 如果超出范围就返回0
// 只允许一个访问
func GetPortFromHost(host *Host) int {
	portMutex.Lock()
	defer portMutex.Unlock()
	newPort := 49000

	ports := host.Ports()
	logger.Debug("ports are: ", ports)
	length := len(ports)
	if length > 0 {
		var i int
		for i = 1; i < length; i = i + 1 {
			tmpPort := ports[i-1]
			if tmpPort+1 != ports[i] {
				newPort = tmpPort + 1
				break
			}
		}
		if i == length {
			newPort = ports[i-1] + 1
		}
	}

	if newPort >= 50000 {
		return 0
	} else {
		host.AddPort(newPort)
	}

	return newPort
}
