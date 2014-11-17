package models

import (
	"../config"
	. "../utils"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	"path"
	"time"
)

var (
	AppPathPrefix       = "/NBE/"
	ShouldNotBeDIR      = errors.New("should not be dir")
	NoKeyFound          = errors.New("no key found")
	NoResourceFound     = errors.New("no resource found")
	AlreadyHaveResource = errors.New("already have this resource")
)

type Application struct {
	Id        int
	Name      string
	Version   string
	Pname     string
	User      *User `orm:"rel(fk)"`
	Group     string
	Created   time.Time `orm:"auto_now_add;type(datetime)"`
	ImageAddr string
}

type AppYaml struct {
	Appname  string   `json:appname`
	Runtime  string   `json:runtime`
	Port     int      `json:port`
	Cmd      []string `json:cmd`
	Daemon   []string `json:daemon`
	Test     []string `json:test`
	Build    []string `json:build`
	Services []string `json:services`
	Static   string   `json:static`
	Schema   string   `json:schema`
}

type ConfigYaml map[string]map[string]interface{}

// Application
func (self *Application) TableUnique() [][]string {
	return [][]string{
		[]string{"Name", "Version"},
	}
}

func GetApplicationById(appId int) *Application {
	var app Application
	if err := db.QueryTable(new(Application)).Filter("Id", appId).One(&app); err != nil {
		return nil
	}
	return &app
}

func NewApplication(projectname, version, group, appyaml string) *Application {
	var appYamlDict AppYaml

	if err := JSONDecode(appyaml, &appYamlDict); err != nil {
		Logger.Info("app.yaml error: ", err)
		return nil
	}
	Logger.Debug("app.yaml: ", appYamlDict)

	appName := appYamlDict.Appname
	if app := GetApplicationByNameAndVersion(appName, version); app != nil {
		Logger.Info("App already registered: ", app)
		return app
	}

	// 生成新用户
	user := NewUser(appName)
	if user == nil {
		return nil
	}

	// 用户绑定应用
	app := &Application{Name: appName, Version: version, Pname: projectname, Group: group, User: user}
	if _, err := db.Insert(app); err != nil {
		Logger.Info("Create App error: ", err)
		return nil
	}

	if appYaml, err := YAMLEncode(appYamlDict); err == nil {
		etcdClient.Create(app.GetYamlPath("app"), appYaml, 0)
	}
	return app
}

func GetApplicationByNameAndVersion(name, version string) *Application {
	var app Application
	if err := db.QueryTable(new(Application)).Filter("Name", name).Filter("Version", version).RelatedSel().One(&app); err != nil {
		return nil
	}
	return &app
}

func (self *Application) CreateDNS() error {
	dns := map[string]string{
		"host": config.Config.Masteraddr,
	}
	p := path.Join("/skydns/com/hunantv/intra", self.Name)
	_, err := etcdClient.Create(p, "", 0)
	if err != nil {
		return err
	}
	r, err := JSONEncode(dns)
	if err != nil {
		return err
	}
	etcdClient.Set(p, r, 0)
	return nil
}

func (self *Application) GetYamlPath(cpath string) string {
	return path.Join(AppPathPrefix, self.Name, self.Version, fmt.Sprintf("%s.yaml", cpath))
}

func (self *Application) GetAppYaml() (*AppYaml, error) {
	var appYaml AppYaml
	cpath := self.GetYamlPath("app")
	r, err := etcdClient.Get(cpath, false, false)
	if err != nil {
		return nil, err
	}
	if r.Node.Dir {
		return nil, ShouldNotBeDIR
	}
	if err := YAMLDecode(r.Node.Value, &appYaml); err != nil {
		return nil, err
	}
	return &appYaml, nil
}

func (self *Application) UserUid() int {
	return self.User.Id
}

func (self *Application) SetImageAddr(addr string) {
	self.ImageAddr = addr
	db.Update(self)
}

func (self *Application) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("AppId", self.Id).OrderBy("Port").All(&cs)
	return cs
}

func (self *Application) AllVersionHosts() []*Host {
	var rs orm.ParamsList
	var hosts []*Host
	_, err := db.Raw("SELECT distinct(host_id) FROM container WHERE app_name=?", self.Name).ValuesFlat(&rs)
	if err == nil && len(rs) > 0 {
		db.QueryTable(new(Host)).Filter("id__in", rs).All(&hosts)
	}
	return hosts
}

// env could be prod/test
func resourceKey(name, env string) string {
	if env != "prod" && env != "test" {
		return ""
	}
	return path.Join(AppPathPrefix, name, fmt.Sprintf("resource-%s", env))
}

func resource(name, env string) map[string]map[string]interface{} {
	p := resourceKey(name, env)
	if p == "" {
		return nil
	}
	r, err := etcdClient.Get(p, false, false)
	if err != nil {
		return nil
	}
	if r.Node.Dir {
		return nil
	}
	var d map[string]map[string]interface{}
	YAMLDecode(r.Node.Value, &d)
	return d
}

func (self *Application) Resource(env string) map[string]map[string]interface{} {
	return resource(self.Name, env)
}

func (self *Application) MySQLDSN(env, key string) string {
	r := self.Resource(env)
	if r == nil {
		return ""
	}
	mysql, exists := r[key]
	if !exists {
		return ""
	}
	return fmt.Sprintf("%v@%v@tcp(%v:%v)/%v?autocommit=true",
		mysql["username"], mysql["password"], mysql["host"], mysql["port"], mysql["db"])
}

func SetHookBranch(name, branch string) error {
	p := path.Join(AppPathPrefix, name, "hookbranch")
	_, err := etcdClient.Set(p, branch, 0)
	if err != nil {
		return err
	}
	return nil
}

func GetHookBranch(name string) (string, error) {
	p := path.Join(AppPathPrefix, name, "hookbranch")
	r, err := etcdClient.Get(p, false, false)
	if err != nil {
		return "", err
	}
	if r.Node.Dir {
		return "", ShouldNotBeDIR
	}
	return r.Node.Value, nil
}

func AppendResource(name, env, key string, res map[string]interface{}) error {
	p := resourceKey(name, env)
	if p == "" {
		return NoKeyFound
	}
	r := resource(name, env)
	if r == nil {
		r = make(map[string]map[string]interface{})
	}
	_, exists := r[key]
	if exists {
		return AlreadyHaveResource
	}
	r[key] = res
	y, err := YAMLEncode(r)
	if err != nil {
		return err
	}
	_, err = etcdClient.Set(p, y, 0)
	if err != nil {
		return err
	}
	return nil
}
