package models

import (
	"../config"
	. "../utils"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	"path"
	"strings"
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
	ID        int         `orm:"auto;pk;column(id)" json:"id"`
	Name      string      `json:"name"`
	Pname     string      `json:"pname"`
	Namespace string      `json:"namespace"`
	User      *User       `orm:"rel(fk)" json:"-"`
	Manager   *ManagerSet `orm:"-" json:"-"`
}

type AppVersion struct {
	ID        int       `orm:"auto;pk;column(id)" json:"id"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Created   time.Time `orm:"auto_now_add;type(datetime)" json:"created"`
	ImageAddr string    `json:"image_addr"`
	AppYaml   *AppYaml  `orm:"-" json:"app.yaml"`
}

type AppYaml struct {
	Appname        string   `json:"appname"`
	Runtime        string   `json:"runtime"`
	Port           int      `json:"port"`
	Cmd            []string `json:"cmd"`
	Daemon         []string `json:"daemon"`
	Test           []string `json:"test"`
	Build          []string `json:"build"`
	Static         string   `json:"static"`
	Schema         string   `json:"schema"`
	ReleaseManager []string `json:"release_manager" yaml:"release_manager"`
}

type ManagerSet struct {
	appname string
	manager map[string]struct{}
}

func NewManagerSet(appname string) *ManagerSet {
	m := ManagerSet{appname: appname, manager: map[string]struct{}{}}
	p := path.Join(AppPathPrefix, appname, "release_manager")
	r, err := etcdClient.Get(p, false, false)
	if err != nil || r.Node.Dir {
		return &m
	}
	names := strings.Split(r.Node.Value, "\n")
	for _, name := range names {
		m.manager[name] = struct{}{}
	}
	return &m
}

func (self *ManagerSet) SetManager(names []string) error {
	m := strings.Join(names, "\n")
	p := path.Join(AppPathPrefix, self.appname, "release_manager")
	_, err := etcdClient.Set(p, m, 0)
	if err != nil {
		return err
	}
	return nil
}

func (self *ManagerSet) IsManager(name string) bool {
	if name == "NBEBot" {
		return true
	}
	if len(self.manager) == 0 {
		return true
	}
	_, exists := self.manager[name]
	return exists
}

func GetApplication(appname string) *Application {
	var app Application
	if err := db.QueryTable(new(Application)).Filter("Name", appname).RelatedSel().One(&app); err != nil {
		return nil
	}
	app.Manager = NewManagerSet(app.Name)
	return &app
}

func GetAllApplications(start, limit int) []*Application {
	var apps []*Application
	db.QueryTable(new(Application)).OrderBy("Name").Limit(limit, start).All(&apps)
	return apps
}

func GetVersion(appname, version string) *AppVersion {
	var v AppVersion
	err := db.QueryTable(new(AppVersion)).Filter("Name", appname).Filter("Version", version).One(&v)
	if err != nil {
		return nil
	}
	v.AppYaml, _ = v.GetAppYaml()
	return &v
}

func GetVersions(appname string, start, limit int) []*AppVersion {
	var vs []*AppVersion
	db.QueryTable(new(AppVersion)).Filter("Name", appname).OrderBy("-ID").Limit(limit, start).All(&vs)
	return vs
}

func GetVersionByID(id int) *AppVersion {
	var v AppVersion
	err := db.QueryTable(new(AppVersion)).Filter("ID", id).One(&v)
	if err != nil {
		return nil
	}
	v.AppYaml, _ = v.GetAppYaml()
	return &v
}

func newApplication(appname, projectname, namespace string) *Application {
	user := NewUser(appname)
	if user == nil {
		return nil
	}
	app := &Application{Name: appname, Pname: projectname, Namespace: namespace, User: user}
	_, id, err := db.ReadOrCreate(app, "Name")
	if err != nil {
		return nil
	}
	app.ID = int(id)
	return app
}

func newVersion(appname, version string) *AppVersion {
	v := &AppVersion{Name: appname, Version: version}
	_, id, err := db.ReadOrCreate(v, "Name", "Version")
	if err != nil {
		return nil
	}
	v.ID = int(id)
	return v
}

// 项目名, 版本号, gitlab的ns, yaml的yaml格式, 提交者
func Register(projectname, version, namespace, appyaml, submitter string) *Application {
	var appYamlDict AppYaml

	if err := YAMLDecode(appyaml, &appYamlDict); err != nil {
		Logger.Info("app.yaml error: ", err)
		return nil
	}
	Logger.Debug("app.yaml: ", appYamlDict)

	appname := appYamlDict.Appname
	// 设置新的release manager
	m := NewManagerSet(appname)
	if !m.IsManager(submitter) {
		Logger.Info("current user is ", submitter, " not manager")
		return nil
	}

	m.SetManager(appYamlDict.ReleaseManager)

	app := newApplication(appname, projectname, namespace)
	if app == nil {
		Logger.Info("app create failed")
		return nil
	}
	av := newVersion(appname, version)
	if av == nil {
		Logger.Info("version create failed")
		return nil
	}
	app.Manager = NewManagerSet(appname)

	// create test/prod empty yaml file for levi
	etcdClient.Create(resourceKey(appname, "test"), "", 0)
	etcdClient.Create(resourceKey(appname, "prod"), "", 0)
	etcdClient.Create(av.GetYamlPath("app"), appyaml, 0)
	return app
}

func (a *Application) CreateDNS() error {
	dns := map[string]string{
		"host": config.Config.Masteraddr,
	}
	p := path.Join("/skydns/com/hunantv/intra", a.Name)
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

func (av *AppVersion) GetYamlPath(p string) string {
	return path.Join(AppPathPrefix, av.Name, av.Version, fmt.Sprintf("%s.yaml", p))
}

func (av *AppVersion) GetAppYaml() (*AppYaml, error) {
	var appYaml AppYaml
	p := av.GetYamlPath("app")
	r, err := etcdClient.Get(p, false, false)
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

func (av *AppVersion) UserUID() int {
	app := GetApplication(av.Name)
	return app.UserUID()
}

func (a *Application) UserUID() int {
	return a.User.ID
}

func (av *AppVersion) SetImageAddr(addr string) {
	av.ImageAddr = addr
	db.Update(av)
}

func (a *Application) AllVersions(start, limit int) []*AppVersion {
	var avs []*AppVersion
	db.QueryTable(new(AppVersion)).Filter("Name", a.Name).OrderBy("-ID").Limit(limit, start).All(&avs)
	return avs
}

func (a *Application) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("AppName", a.Name).OrderBy("Port").All(&cs)
	return cs
}

func (av *AppVersion) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("AppName", av.Name).Filter("Version", av.Version).OrderBy("Port").All(&cs)
	return cs
}

func (a *Application) AllVersionHosts() []*Host {
	var rs orm.ParamsList
	var hosts []*Host
	_, err := db.Raw("SELECT distinct(host_id) FROM container WHERE app_name=?", a.Name).ValuesFlat(&rs)
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

func (a *Application) Resource(env string) map[string]map[string]interface{} {
	return resource(a.Name, env)
}

func (a *Application) MySQLDSN(env, key string) string {
	r := a.Resource(env)
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

func (a *Application) IsManager(name string) bool {
	return a.Manager.IsManager(name)
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

func RemoveResource(name, env, key string) error {
	p := resourceKey(name, env)
	if p == "" {
		return NoKeyFound
	}
	r := resource(name, env)
	if r == nil {
		return NoResourceFound
	}
	delete(r, key)
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
