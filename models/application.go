package models

import (
	"../config"
	. "../utils"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

const (
	appPathPrefix = "/NBE/"
)

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
	Daemon   bool     `json:daemon`
}

type ConfigYaml map[string]interface{}

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
		Logger.Debug("app.yaml error: ", err1)
		Logger.Debug("config.yaml error: ", err2)
		return nil
	}
	Logger.Debug("app.yaml: ", appYamlJson)
	Logger.Debug("config.yaml: ", oconfigYamlJson)

	for k, v := range oconfigYamlJson {
		copyConfigYamlJson[k] = v
		testConfigYamlJson[k] = v
	}

	// 生成新用户
	appName := appYamlJson.Appname

	if app := GetApplicationByNameAndVersion(appName, Version); app != nil {
		// 已经有就不注册了
		return app
	}

	user := User{Name: appName}
	if _, id, err := db.ReadOrCreate(&user, "Name"); err == nil {
		user.Id = int(id)
	} else {
		Logger.Info("create user failed", err)
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
			if dbInfoString := app.GetOrCreateDbInfo("mysql", CreateMySQL); dbInfoString != "" {
				var d map[string]interface{}
				if err := JSONDecode(dbInfoString, &d); err == nil {
					oconfigYamlJson["mysql"] = d
					// 没得可以分配的, 先写这个吧, 一定会挂
					testConfigYamlJson["mysql"] = d
				} else {
					Logger.Info("mysql create failed")
				}
			}
		}
		if service == "redis" {
			if dbInfoString := app.GetOrCreateDbInfo("redis", CreateRedis); dbInfoString != "" {
				var d map[string]interface{}
				if err := JSONDecode(dbInfoString, &d); err == nil {
					oconfigYamlJson["redis"] = d
					// 没得可以分配的, 先写这个吧, 一定会挂
					testConfigYamlJson["redis"] = d
				} else {
					Logger.Info("redis create failed")
				}
			}
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
	etcdClient.CreateDir(path.Join(appPathPrefix, "_Apps", app.Name, "tests"), 0)

	return &app
}

func GetApplicationByNameAndVersion(name, version string) *Application {
	var app Application
	if err := db.QueryTable(new(Application)).Filter("Name", name).Filter("Version", version).RelatedSel().One(&app); err != nil {
		return nil
	}
	return &app
}

func (self *Application) GetOrCreateDbInfo(kind string, createFunction func(*Application) (map[string]interface{}, error)) string {
	cpath := path.Join(appPathPrefix, self.Name, kind)
	if _, err := etcdClient.Create(cpath, "", 0); err == nil {
		db, err := createFunction(self)
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

func (self *Application) CreateDNS() error {
	dns := make(map[string]string)
	dns["host"] = config.Config.Masteraddr
	cpath := path.Join("/skydns/com/hunantv/intra", self.Name)
	if _, err := etcdClient.Create(cpath, "", 0); err != nil {
		return err
	}
	if r, err := JSONEncode(dns); err == nil {
		etcdClient.Set(cpath, r, 0)
		return nil
	} else {
		return err
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

func CreateMySQL(app *Application) (map[string]interface{}, error) {

	password := CreateSha1HexValue([]byte(app.Name + app.Version + time.Now().String()))

	form := url.Values{}
	form.Set("SysUid", config.Config.Dba.Sysuid)
	form.Set("SysPwd", config.Config.Dba.Syspwd)
	form.Set("businessCode", config.Config.Dba.Bcode)
	form.Set("DbName", app.Name)
	form.Set("DbUid", app.Name)
	form.Set("DbPwd", password[:8])

	if r, err := http.DefaultClient.PostForm(config.Config.Dba.Addr, form); err == nil {
		defer r.Body.Close()
		result, _ := ioutil.ReadAll(r.Body)
		var d map[string]string
		json.Unmarshal(result, &d)
		if d["Result"] == "0" {
			return nil, errors.New("create mysql failed")
		}
		ret := make(map[string]interface{})
		ret["username"] = d["DbUser"]
		ret["password"] = d["DbPwd"]
		ret["host"] = d["IPAddress"]
		ret["port"], _ = strconv.Atoi(d["Port"])
		ret["db"] = d["DbName"]
		return ret, nil
	} else {
		return nil, err
	}
}

func CreateRedis(app *Application) (map[string]interface{}, error) {
	// TODO 接入redis
	r := make(map[string]interface{})
	r["host"] = "10.1.201.88"
	r["port"] = time.Now().Nanosecond()%13 + 2000
	return r, nil
}
