package main

import (
	"./models"
	"./resources"
	"./utils"
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

func JSONWrapper(f func(*Request) interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		r := NewRequest(req)
		json.NewEncoder(w).Encode(f(r))
	}
}

func EchoHandler(req *Request) interface{} {
	msg := req.Form.Get("msg")
	return JSON{
		"r":     0,
		"msg":   msg,
		"start": req.Start,
		"limit": req.Limit,
		"user":  req.User,
	}
}

func RegisterApplicationHandler(req *Request) interface{} {
	projectname := req.URL.Query().Get(":projectname")
	version := req.URL.Query().Get(":version")
	group := req.Form.Get("group")
	appyaml := req.Form.Get("appyaml")

	app := models.Register(projectname, version, group, appyaml, req.User)
	if app == nil {
		return JSON{"r": 1, "msg": "register app fail"}
	}
	return JSON{"r": 0, "msg": "ok"}
}

func AddContainerHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")
	daemon := req.Form.Get("daemon")

	av := models.GetVersion(name, version)
	host := models.GetHostByIP(ip)

	if av == nil || host == nil {
		return NoSuchApp
	}
	if appyaml, err := av.GetAppYaml(); err != nil || (daemon == "true" && len(appyaml.Daemon) == 0) {
		return JSON{"r": 1, "msg": "daemon set true but no daemon defined"}
	}
	task := models.AddContainerTask(av, host, daemon == "true")
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.ID}
}

func BuildImageHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	// 暂时没有monitor, 那么人肉指定host吧
	ip := req.Form.Get("host")

	av := models.GetVersion(name, version)
	host := models.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}
	base := req.Form.Get("base")
	task := models.BuildImageTask(av, base)
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.ID}
}

func TestImageHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	// 暂时没有monitor, 那么人肉指定host吧
	ip := req.Form.Get("host")

	av := models.GetVersion(name, version)
	host := models.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}
	task := models.TestApplicationTask(av, host)
	err := hub.Dispatch(host.IP, task)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_id": task.ID}
}

func DeployApplicationHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ips := req.Form["hosts"]
	daemon := req.Form.Get("daemon")

	av := models.GetVersion(name, version)
	hosts := models.GetHostsByIPs(ips)
	if av == nil {
		return NoSuchApp
	}
	if appyaml, err := av.GetAppYaml(); err != nil || (daemon == "true" && len(appyaml.Daemon) == 0) {
		return JSON{"r": 1, "msg": "no daemon defined"}
	}
	taskIds, err := DeployApplicationHelper(av, hosts, daemon == "true")
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func RemoveApplicationHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")

	av := models.GetVersion(name, version)
	host := models.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}
	taskIds, err := RemoveApplicationFromHostHelper(av, host)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func UpdateApplicationHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	fromVersion := req.URL.Query().Get(":version")

	ips := req.Form["hosts"]
	toVersion := req.Form.Get("to")

	from := models.GetVersion(name, fromVersion)
	to := models.GetVersion(name, toVersion)
	hosts := models.GetHostsByIPs(ips)
	if from == nil || to == nil {
		return JSON{"r": 1, "msg": fmt.Sprintf("no such app %v, %v", from, to)}
	}
	taskIds, err := UpdateApplicationHelper(from, to, hosts)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func RemoveContainerHandler(req *Request) interface{} {
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
	return JSON{"r": 0, "msg": "ok", "task_id": task.ID}
}

func NewMySQLInstanceHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	mysqlName := req.Form.Get("name")
	env := req.Form.Get("env")

	if mysqlName == "" {
		mysqlName = "mysql"
	}

	if app := models.GetApplication(name); app == nil {
		return NoSuchApp
	}

	var dbName string
	switch env {
	case "test":
		dbName = fmt.Sprintf("%s_test", name)
	case "prod":
		dbName = name
	default:
		dbName = ""
	}
	if dbName == "" {
		return JSON{"r": 1, "msg": "env must be test/prod", "mysql": nil}
	}

	mysql, err := resources.NewMySQLInstance(dbName, name)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "mysql": nil}
	}
	err = models.AppendResource(name, env, mysqlName, mysql)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "mysql": nil}
	}
	return JSON{"r": 0, "msg": "", "mysql": mysql}
}

func NewRedisInstanceHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	redisName := req.Form.Get("name")
	env := req.Form.Get("env")

	if redisName == "" {
		redisName = "redis"
	}

	if app := models.GetApplication(name); app == nil {
		return NoSuchApp
	}

	var dbName string
	switch env {
	case "test":
		dbName = fmt.Sprintf("%s_test$%s", name, redisName)
	case "prod":
		dbName = fmt.Sprintf("%s$%s", name, redisName)
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
	err = models.AppendResource(name, env, redisName, redis)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "redis": nil}
	}
	return JSON{"r": 0, "msg": "", "redis": redis}
}

func RemoveResourceHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	key := req.Form.Get("name")
	env := req.Form.Get("env")

	if app := models.GetApplication(name); app == nil {
		return NoSuchApp
	}
	err := models.RemoveResource(name, env, key)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok"}
}

func SyncDBHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	schema := req.Form.Get("schema")

	r := JSON{"r": 1, "msg": ""}
	app := models.GetApplication(name)
	if app == nil {
		r["msg"] = fmt.Sprintf("app %s, %s not found", name)
		return r
	}
	dsn := app.MySQLDSN("prod", "mysql")
	if dsn == "" {
		r["msg"] = fmt.Sprintf("app %s, %s has no dsn", name)
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

func AppBranchHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	if app := models.GetApplication(name); app == nil {
		return NoSuchApp
	}
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

func GetAllApplications(req *Request) interface{} {
	return models.GetAllApplications(req.Start, req.Limit)
}

func GetApplication(req *Request) interface{} {
	return models.GetApplication(req.URL.Query().Get(":app"))
}

func GetAppContainers(req *Request) interface{} {
	app := models.GetApplication(req.URL.Query().Get(":app"))
	if app == nil {
		return []*models.Container{}
	}
	return app.Containers()
}

func GetAppVersions(req *Request) interface{} {
	return models.GetVersions(req.URL.Query().Get(":app"), req.Start, req.Limit)
}

func GetAppJobs(req *Request) interface{} {
	status := utils.Atoi(req.URL.Query().Get("status"), -1)
	succ := utils.Atoi(req.URL.Query().Get("succ"), -1)
	name := req.URL.Query().Get(":app")
	return models.GetJobs(name, "", status, succ, req.Start, req.Limit)
}

func GetAppVersionJobs(req *Request) interface{} {
	status := utils.Atoi(req.URL.Query().Get("status"), -1)
	succ := utils.Atoi(req.URL.Query().Get("succ"), -1)
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	return models.GetJobs(name, version, status, succ, req.Start, req.Limit)
}

func GetAppVersionContainers(req *Request) interface{} {
	av := models.GetVersion(req.URL.Query().Get(":app"), req.URL.Query().Get(":version"))
	if av == nil {
		return []*models.Container{}
	}
	return av.Containers()
}

func GetAppVersion(req *Request) interface{} {
	return models.GetVersion(req.URL.Query().Get(":app"), req.URL.Query().Get(":version"))
}

func GetHostByID(req *Request) interface{} {
	return models.GetHostByID(utils.Atoi(req.URL.Query().Get(":id"), 0))
}

func GetAllHosts(req *Request) interface{} {
	return models.GetAllHosts(req.Start, req.Limit)
}

func GetContainerByCid(req *Request) interface{} {
	return models.GetContainerByCid(req.URL.Query().Get(":cid"))
}

func GetContainers(req *Request) interface{} {
	hostID := utils.Atoi(req.URL.Query().Get("host_id"), -1)
	return models.GetContainers(hostID, req.URL.Query().Get("name"),
		req.URL.Query().Get("version"), req.Start, req.Limit)
}

func init() {
	RestServer = pat.New()

	rs := map[string]map[string]func(*Request) interface{}{
		"POST": {
			"/app/:projectname/:version":     RegisterApplicationHandler,
			"/app/:app/:version/add":         AddContainerHandler,
			"/app/:app/:version/build":       BuildImageHandler,
			"/app/:app/:version/test":        TestImageHandler,
			"/app/:app/:version/deploy":      DeployApplicationHandler,
			"/app/:app/:version/update":      UpdateApplicationHandler,
			"/app/:app/:version/remove":      RemoveApplicationHandler,
			"/container/:cid/remove":         RemoveContainerHandler,
			"/resource/:app/mysql":           NewMySQLInstanceHandler,
			"/resource/:app/:version/syncdb": SyncDBHandler,
			"/resource/:app/redis":           NewRedisInstanceHandler,
			"/resource/:app/remove":          RemoveResourceHandler,
		},
		"GET": {
			"/echo":                         EchoHandler,
			"/app":                          GetAllApplications,
			"/app/:app":                     GetApplication,
			"/app/:app/branch":              AppBranchHandler,
			"/app/:app/jobs":                GetAppJobs,
			"/app/:app/containers":          GetAppContainers,
			"/app/:app/versions":            GetAppVersions,
			"/app/:app/:version":            GetAppVersion,
			"/app/:app/:version/jobs":       GetAppVersionJobs,
			"/app/:app/:version/containers": GetAppVersionContainers,
			"/host/:id":                     GetHostByID,
			"/hosts":                        GetAllHosts,
			"/container/:cid":               GetContainerByCid,
			"/containers":                   GetContainers,
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
