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
	appPathPrefix = "/nbe/app/"
)

var db orm.Ormer
var etcdClient *etcd.Client
var portMutex sync.Mutex

type Host struct {
	Id   int
	Ip   string
	Name string
}

type Container struct {
	Id          string
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
	Appname string   `json:appname`
	Runtime string   `json:runtime`
	Cmd     []string `json:cmd`
	Build   []string `json:build`
}

type ConfigYaml map[string]interface{}

// models 初始化
// 连接 mysql
// 连接 etcd
// 可能还需要连接 redis
func init() {
	// mysql
	// TODO 改成参数配置
	orm.RegisterDataBase("default", "mysql", "root:@/dot?charset=utf8", 30)
	orm.RegisterModel(new(Application), new(User), new(Host))
	orm.RunSyncdb("default", true, true)
	db = orm.NewOrm()

	// etcd
	etcdClient = etcd.NewClient([]string{"http://localhost:4001", "http://localhost:4002"})
	etcdClient.SyncCluster()

	// Mutex
	portMutex = syncMutex{}
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
	if err := JSONDecode(appyaml, &appYamlJson); err != nil {
		fmt.Println("app.yaml ", err, appyaml)
		return nil
	}

	// 生成新用户
	appName := appYamlJson.Appname
	user := User{Name: appName}
	if _, id, err := db.ReadOrCreate(&user, "Name"); err == nil {
		user.Id = int(id)
	} else {
		return nil
	}

	// 用户绑定应用
	app := Application{Name: appName, Version: version, Pname: projectname, User: &user}
	if _, err := db.Insert(&app); err != nil {
		return nil
	}

	// 保存配置文件
	etcdClient.Create((&app).GetYamlPath("app"), appyaml, 0)
	etcdClient.Create((&app).GetYamlPath("config"), configyaml, 0)

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

func (self *Application) GetYamlPath(cpath string) string {
	return path.Join(appPathPrefix, self.Name, self.Version, cpath+".yaml")
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
	if err = JSONDecode(r.Node.Value, &appYaml); err != nil {
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
	if err = JSONDecode(r.Node.Value, &configYaml); err != nil {
		return &configYaml, err
	}
	return &configYaml, nil
}

func (self *Application) UserUid() int {
	return self.User.Id
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
		[]string{"Ip"},
	}
}

func NewHost(ip, name string) *Host {
	host := Host{Ip: ip, Name: name}
	if _, id, err := db.ReadOrCreate(&host, "Ip"); err == nil {
		host.Id = int(id)
		return &host
	}
	return nil
}

func GetHostById(hostId int) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("Id", hostId).one(&host)
	if err != nil {
		return nil
	}
	return &host
}

func GetHostByIp(ip string) *Host {
	var host Host
	err := db.QueryTable(new(Host)).Filter("Ip", ip).one(&host)
	if err != nil {
		return nil
	}
	return &host
}

func (self *Host) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("HostId", self.Id).OrderBy("Port").All(&cs)
	return cs
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

func NewContainer(app *Application, host *Host, port int, containerId, daemonId string) *Container {
	c := Container{Port: port, ContainerId: containerId, DaemonId: daemonId, AppId: app.Id, HostId: host.Id}
	if _, err := db.Insert(&c); err == nil {
		return &c
	}
	return nil
}

// 获取一个host上的可用的一个端口
// 如果超出范围就返回0
// 只允许一个访问
func GetPortFromHost(host *Host) int {
	portMutex.Lock()
	newPort := 49000

	cs := host.Containers()
	length := len(cs)
	if length > 0 {
		var i int
		for i = 1; i < length; i = i + 1 {
			tmpPort := cs[0].Port
			if tmpPort+1 != cs[i].Port {
				newPort = tmpPort + 1
				break
			}
		}
		if i == length {
			newPort = cs[i-1].Port + 1
		}
	}
	portMutex.Unlock()

	if newPort >= 50000 {
		return 0
	}

	return newPort
}
