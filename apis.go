package main

import (
	"./models"
	"./resources"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bmizerany/pat"
)

var RestServer *pat.PatternServeMux

type JSON map[string]interface{}

var (
	NoSuchApp       = JSON{"r": 1, "msg": "no such app"}
	NoSuchHost      = JSON{"r": 1, "msg": "no such host"}
	NoSuchContainer = JSON{"r": 1, "msg": "no such container"}
)

func JSONWrapper(f func(*http.Request) JSON) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		json.NewEncoder(w).Encode(f(req))
	}
}

func EchoHandler(req *http.Request) JSON {
	msg := req.Form.Get("msg")
	return JSON{"r": 0, "msg": msg}
}

func RegisterApplicationHandler(req *http.Request) JSON {
	projectname := req.URL.Query().Get(":projectname")
	version := req.URL.Query().Get(":version")
	group := req.Form.Get("group")
	appyaml := req.Form.Get("appyaml")

	app := models.NewApplication(projectname, version, group, appyaml)
	if app == nil {
		return JSON{"r": 1, "msg": "register app fail"}
	}
	return JSON{"r": 0, "msg": "ok"}
}

func AddContainerHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")
	daemon := req.Form.Get("daemon")

	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)

	if app == nil || host == nil {
		return NoSuchApp
	}
	if appyaml, err := app.GetAppYaml(); err != nil || (daemon == "true" && len(appyaml.Daemon) == 0) {
		return JSON{"r": 1, "msg": "daemon set true but no daemon defined"}
	}
	task := models.AddContainerTask(app, host, daemon == "true")
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.Id}
}

func BuildImageHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	// 暂时没有monitor, 那么人肉指定host吧
	ip := req.Form.Get("host")

	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)
	if app == nil || host == nil {
		return NoSuchApp
	}
	base := req.Form.Get("base")
	task := models.BuildImageTask(app, base)
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.Id}
}

func TestImageHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	// 暂时没有monitor, 那么人肉指定host吧
	ip := req.Form.Get("host")

	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)
	if app == nil || host == nil {
		return NoSuchApp
	}
	task := models.TestApplicationTask(app, host)
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.Id}
}

func DeployApplicationHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ips := req.Form["hosts"]
	daemon := req.Form.Get("daemon")

	app := models.GetApplicationByNameAndVersion(name, version)
	hosts := models.GetHostsByIPs(ips)
	if app == nil {
		return NoSuchApp
	}
	if appyaml, err := app.GetAppYaml(); err != nil || (daemon == "true" && len(appyaml.Daemon) == 0) {
		return JSON{"r": 1, "msg": "no daemon defined"}
	}
	taskIds, err := DeployApplicationHelper(app, hosts, daemon == "true")
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func RemoveApplicationHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")

	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)
	if app == nil || host == nil {
		return NoSuchApp
	}
	taskIds, err := RemoveApplicationFromHostHelper(app, host)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func UpdateApplicationHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	fromVersion := req.URL.Query().Get(":version")

	ips := req.Form["hosts"]
	toVersion := req.Form.Get("to")

	fromApp := models.GetApplicationByNameAndVersion(name, fromVersion)
	toApp := models.GetApplicationByNameAndVersion(name, toVersion)
	hosts := models.GetHostsByIPs(ips)
	if fromApp == nil || toApp == nil {
		return JSON{"r": 1, "msg": fmt.Sprintf("no such app %v, %v", fromApp, toApp)}
	}
	taskIds, err := UpdateApplicationHelper(fromApp, toApp, hosts)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func RemoveContainerHandler(req *http.Request) JSON {
	cid := req.URL.Query().Get(":cid")

	container := models.GetContainerByCid(cid)
	if container == nil {
		return NoSuchContainer
	}
	host := container.Host()
	task := models.RemoveContainerTask(container)
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.Id}
}

func NewMySQLInstanceHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	mysqlName := req.Form.Get("name")
	env := req.Form.Get("env")

	if mysqlName == "" {
		mysqlName = "mysql"
	}

	app := models.GetApplicationByNameAndVersion(name, version)
	if app == nil {
		return NoSuchApp
	}

	var dbName string
	switch env {
	case "test":
		dbName = fmt.Sprintf("%s_test", app.Name)
	case "prod":
		dbName = app.Name
	default:
		dbName = ""
	}
	if dbName == "" {
		return JSON{"r": 1, "msg": "env must be test/prod", "mysql": nil}
	}

	mysql, err := resources.NewMySQLInstance(dbName, app.Name)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "mysql": nil}
	}
	err = models.AppendResource(app.Name, env, mysqlName, mysql)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "mysql": nil}
	}
	return JSON{"r": 0, "msg": "", "mysql": mysql}
}

func NewRedisInstanceHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	redisName := req.Form.Get("name")
	env := req.Form.Get("env")

	if redisName == "" {
		redisName = "redis"
	}

	app := models.GetApplicationByNameAndVersion(name, version)
	if app == nil {
		return NoSuchApp
	}

	var dbName string
	switch env {
	case "test":
		dbName = fmt.Sprintf("%s_test$%s", app.Name, redisName)
	case "prod":
		dbName = fmt.Sprintf("%s$%s", app.Name, redisName)
	default:
		dbName = ""
	}
	if dbName == "" {
		return JSON{"r": 1, "msg": "env must be test/prod", "redis": nil}
	}

	redis, err := resources.NewRedisInstance(dbName)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "redis": nil}
	}
	err = models.AppendResource(app.Name, env, redisName, redis)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "redis": nil}
	}
	return JSON{"r": 0, "msg": "", "redis": redis}
}

func SyncDBHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	schema := req.Form.Get("schema")

	r := JSON{"r": 1, "msg": ""}
	app := models.GetApplicationByNameAndVersion(name, version)
	if app == nil {
		r["msg"] = fmt.Sprintf("app %s, %s not found", name, version)
		return r
	}
	dsn := app.MySQLDSN("prod", "mysql")
	if dsn == "" {
		r["msg"] = fmt.Sprintf("app %s, %s has no dsn", name, version)
		return r
	}
	err := resources.SyncSchema(dsn, schema)
	if err != nil {
		r["msg"] = err.Error()
		return r
	}
	r["r"] = 0
	return r
}

func AppBranchHandler(req *http.Request) JSON {
	name := req.URL.Query().Get(":name")
	if req.Method == "PUT" {
		branch := req.Form.Get("branch")
		err := models.SetHookBranch(name, branch)
		if err != nil {
			return JSON{"r": 1, "msg": err.Error()}
		}
		return JSON{"r": 0, "msg": "ok"}
	}
	if req.Method == "GET" {
		branch, err := models.GetHookBranch(name)
		if err != nil {
			return JSON{"r": 1, "msg": err.Error(), "branch": ""}
		}
		return JSON{"r": 0, "msg": "", "branch": branch}
	}
	return JSON{"r": 1, "msg": "method not allowed"}
}

func init() {
	RestServer = pat.New()

	rs := map[string]map[string]func(*http.Request) JSON{
		"POST": {
			"/app/:projectname/:version":     RegisterApplicationHandler,
			"/app/:app/:version/add":         AddContainerHandler,
			"/app/:app/:version/build":       BuildImageHandler,
			"/app/:app/:version/test":        TestImageHandler,
			"/app/:app/:version/deploy":      DeployApplicationHandler,
			"/app/:app/:version/update":      UpdateApplicationHandler,
			"/app/:app/:version/remove":      RemoveApplicationHandler,
			"/container/:cid/remove":         RemoveContainerHandler,
			"/resource/:app/:version/mysql":  NewMySQLInstanceHandler,
			"/resource/:app/:version/syncdb": SyncDBHandler,
			"/resource/:app/:version/redis":  NewRedisInstanceHandler,
		},
		"GET": {
			"/echo":            EchoHandler,
			"/app/:app/branch": AppBranchHandler,
		},
		"PUT": {
			"/app/:app/branch": AppBranchHandler,
		},
	}

	for method, routes := range rs {
		for route, handler := range routes {
			RestServer.Add(method, route, http.HandlerFunc(JSONWrapper(handler)))
		}
	}
}
