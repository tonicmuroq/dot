package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
	"path"
)

const (
	appPathPrefix = "/nbe/app/"
)

// etcdClient

var db orm.Ormer

type Task struct {
	Uuid        string
	TaskType    int
	AppName     string
	AppVersion  string
	Host        string
	ContainerId string
}

type Host struct {
	Id   int
	Ip   string
	Name string
}

type Container struct {
	Id         string
	AppName    string
	AppVersion string
	Host       *Host `orm:"ref(fk)"`
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

// ORM
func init() {
	// TODO 改成参数配置
	orm.RegisterDataBase("default", "mysql", "root:@/dot?charset=utf8", 30)
	orm.RegisterModel(new(Application))
	orm.RegisterModel(new(User))
	orm.RunSyncdb("default", true, true)
	db = orm.NewOrm()
}

// Application
func (self *Application) TableUnique() [][]string {
	return [][]string{
		[]string{"Name", "Version"},
	}
}

func NewApplication(projectname, version, appyaml, configyaml string) *Application {
	// 调整yaml
	if configyaml == "" {
		configyaml = "{}"
	}
	var appYamlJson AppYaml
	if err := json.Unmarshal([]byte(appyaml), &appYamlJson); err != nil {
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
	if err = json.Unmarshal(r.Node.Value, &appYaml); err != nil {
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
	if err = json.Unmarshal(r.Node.Value, &configYaml); err != nil {
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

// Container
func (self *Container) TableIndex() [][]string {
	return [][]string{
		[]string{"AppId"},
		[]string{"ContainerId"},
		[]string{"host_id"}, /* TODO 有点tricky */
	}
}

func NewContainer(app *Application, host *Host) *Container {
	c := Container{AppName: app.Name, AppVersion: app.Version, Host: host}
	if _, err := db.Insert(&c); err == nil {
		return &c
	}
	return nil
}
