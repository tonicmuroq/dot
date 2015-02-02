package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bmizerany/pat"

	"dot"
	"resources"
	"types"
	"utils"
)

var (
	RestAPIServer   *pat.PatternServeMux
	NoSuchApp       = JSON{"r": 1, "msg": "no such app"}
	NoSuchHost      = JSON{"r": 1, "msg": "no such host"}
	NoSuchContainer = JSON{"r": 1, "msg": "no such container"}
)

type JSON map[string]interface{}

func JSONWrapper(f func(*Request) interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		r := NewRequest(req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(f(r))
	}
}

func EchoHandler(req *Request) interface{} {
	return JSON{
		"r":     0,
		"msg":   req.Form.Get("msg"),
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

	app := types.Register(projectname, version, group, appyaml, req.User)
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
	sub := req.Form.Get("sub_app")

	av := types.GetVersion(name, version)
	host := types.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}

	// if sub is ""
	// will return main app.yaml
	// then the main app will be deployed
	appyaml, err := av.GetSubAppYaml(sub)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}

	if daemon == "true" && len(appyaml.Daemon) == 0 {
		return JSON{"r": 1, "msg": "daemon set true but no daemon defined"}
	}
	task := types.AddContainerTask(av, host, appyaml, daemon == "true")
	err = dot.LeviHub.Dispatch(host.IP, task)
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

	av := types.GetVersion(name, version)
	host := types.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}
	base := req.Form.Get("base")
	task := types.BuildImageTask(av, base)
	err := dot.LeviHub.Dispatch(host.IP, task)
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

	av := types.GetVersion(name, version)
	host := types.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}
	task := types.TestApplicationTask(av, host)
	err := dot.LeviHub.Dispatch(host.IP, task)
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
	sub := req.Form.Get("sub_app")

	av := types.GetVersion(name, version)
	hosts := types.GetHostsByIPs(ips)
	if av == nil {
		return NoSuchApp
	}

	// if sub is ""
	// will return main app.yaml
	// then the main app will be deployed
	appyaml, err := av.GetSubAppYaml(sub)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}

	if daemon == "true" && len(appyaml.Daemon) == 0 {
		return JSON{"r": 1, "msg": "no daemon defined"}
	}

	taskIds, err := dot.DeployApplicationHelper(av, hosts, appyaml, daemon == "true")
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func RemoveApplicationHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")

	av := types.GetVersion(name, version)
	host := types.GetHostByIP(ip)
	if av == nil || host == nil {
		return NoSuchApp
	}
	taskIds, err := dot.RemoveApplicationFromHostHelper(av, host)
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

	from := types.GetVersion(name, fromVersion)
	to := types.GetVersion(name, toVersion)
	hosts := types.GetHostsByIPs(ips)
	if from == nil || to == nil {
		return JSON{"r": 1, "msg": fmt.Sprintf("no such app %v, %v", from, to)}
	}
	taskIds, err := dot.UpdateApplicationHelper(from, to, hosts)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok", "task_ids": taskIds}
}

func RemoveContainerHandler(req *Request) interface{} {
	cid := req.URL.Query().Get(":cid")

	container := types.GetContainerByCid(cid)
	if container == nil {
		return NoSuchContainer
	}
	host := container.Host()
	task := types.RemoveContainerTask(container)
	err := dot.LeviHub.Dispatch(host.IP, task)
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

	if app := types.GetApplication(name); app == nil {
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
	err = types.AppendResource(name, env, mysqlName, mysql)
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

	if app := types.GetApplication(name); app == nil {
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
	err = types.AppendResource(name, env, redisName, redis)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "redis": nil}
	}
	return JSON{"r": 0, "msg": "", "redis": redis}
}

func NewSentryDSNHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	platform := req.Form.Get("platform")

	if platform == "" {
		return JSON{"r": 1, "msg": "no platform defined"}
	}
	if app := types.GetApplication(name); app == nil {
		return NoSuchApp
	}
	sentry, err := resources.NewSentryDSN(name, platform)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "sentry": nil}
	}
	dsn, ok := sentry["dsn"].(string)
	if !ok {
		return JSON{"r": 1, "msg": "sentry not string", "sentry": nil}
	}
	// sentry 是 {"dsn": "udp://xxxx:yyyy@host:port/namespace"}
	err = types.AppendResource(name, "prod", "sentry_dsn", dsn)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "sentry": nil}
	}
	return JSON{"r": 0, "msg": "", "sentry": sentry}
}

func NewInfluxdbHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	app := types.GetApplication(name)
	if app == nil {
		return NoSuchApp
	}
	resource := app.Resource("prod")
	if _, exists := resource["influxdb"]; exists {
		return JSON{"r": 1, "msg": "already has one", "influxdb": nil}
	}
	influxdb, err := resources.NewInfluxdb(name)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "influxdb": nil}
	}
	err = types.AppendResource(name, "prod", "influxdb", influxdb)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error(), "influxdb": nil}
	}
	return JSON{"r": 0, "msg": "ok", "influxdb": influxdb}
}

func RemoveResourceHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	key := req.Form.Get("name")
	env := req.Form.Get("env")

	if app := types.GetApplication(name); app == nil {
		return NoSuchApp
	}
	err := types.RemoveResource(name, env, key)
	if err != nil {
		return JSON{"r": 1, "msg": err.Error()}
	}
	return JSON{"r": 0, "msg": "ok"}
}

func SyncDBHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	schema := req.Form.Get("schema")

	r := JSON{"r": 1, "msg": ""}
	app := types.GetApplication(name)
	if app == nil {
		r["msg"] = fmt.Sprintf("app %s not found", name)
		return r
	}
	dsn := app.MySQLDSN("prod", "mysql")
	if dsn == "" {
		r["msg"] = fmt.Sprintf("app %s has no dsn", name)
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
	if app := types.GetApplication(name); app == nil {
		return NoSuchApp
	}
	if req.Method == "PUT" {
		branch := req.Form.Get("branch")
		err := types.SetHookBranch(name, branch)
		if err != nil {
			return JSON{"r": 1, "msg": err.Error()}
		}
		return JSON{"r": 0, "msg": "ok"}
	}
	if req.Method == "GET" {
		branch, err := types.GetHookBranch(name)
		if err != nil {
			return JSON{"r": 1, "msg": err.Error(), "branch": ""}
		}
		return JSON{"r": 0, "msg": "", "branch": branch}
	}
	return JSON{"r": 1, "msg": "method not allowed"}
}

func AddSubAppYamlHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	appyaml := req.Form.Get("appyaml")
	av := types.GetVersion(name, version)
	if av == nil {
		return NoSuchApp
	}
	var yaml types.AppYaml
	if err := utils.YAMLDecode(appyaml, &yaml); err != nil {
		return JSON{"r": 1, "msg": "not valid yaml file"}
	}
	mainYaml, _ := av.GetAppYaml()
	if mainYaml == nil {
		return JSON{"r": 1, "msg": "no yaml found"}
	}
	if !strings.HasPrefix(yaml.Appname, mainYaml.Appname) {
		return JSON{"r": 1, "msg": "must has the same prefix"}
	}
	av.AddAppYaml(yaml.Appname, appyaml)
	return JSON{"r": 0, "msg": "ok"}
}

func ListSubAppYamlHandler(req *Request) interface{} {
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	av := types.GetVersion(name, version)
	if av == nil {
		return NoSuchApp
	}
	ays, _ := av.ListSubAppYamls()
	return ays
}

func GetAllApplications(req *Request) interface{} {
	return types.GetAllApplications(req.Start, req.Limit)
}

func GetApplication(req *Request) interface{} {
	return types.GetApplication(req.URL.Query().Get(":app"))
}

func GetAppContainers(req *Request) interface{} {
	app := types.GetApplication(req.URL.Query().Get(":app"))
	if app == nil {
		return []*types.Container{}
	}
	return app.Containers()
}

func GetAppVersions(req *Request) interface{} {
	return types.GetVersions(req.URL.Query().Get(":app"), req.Start, req.Limit)
}

func GetAppJobs(req *Request) interface{} {
	status := utils.Atoi(req.URL.Query().Get("status"), -1)
	succ := utils.Atoi(req.URL.Query().Get("succ"), -1)
	name := req.URL.Query().Get(":app")
	return types.GetJobs(name, "", status, succ, req.Start, req.Limit)
}

func GetAppVersionJobs(req *Request) interface{} {
	status := utils.Atoi(req.URL.Query().Get("status"), -1)
	succ := utils.Atoi(req.URL.Query().Get("succ"), -1)
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	return types.GetJobs(name, version, status, succ, req.Start, req.Limit)
}

func GetAppVersionContainers(req *Request) interface{} {
	av := types.GetVersion(req.URL.Query().Get(":app"), req.URL.Query().Get(":version"))
	if av == nil {
		return []*types.Container{}
	}
	return av.Containers()
}

func GetAppVersion(req *Request) interface{} {
	return types.GetVersion(req.URL.Query().Get(":app"), req.URL.Query().Get(":version"))
}

func GetHostByID(req *Request) interface{} {
	return types.GetHostByID(utils.Atoi(req.URL.Query().Get(":id"), 0))
}

func GetAllHosts(req *Request) interface{} {
	return types.GetAllHosts(req.Start, req.Limit)
}

func GetContainerByCid(req *Request) interface{} {
	return types.GetContainerByCid(req.URL.Query().Get(":cid"))
}

func GetContainers(req *Request) interface{} {
	hostID := utils.Atoi(req.URL.Query().Get("host_id"), -1)
	return types.GetContainers(hostID, req.URL.Query().Get("name"),
		req.URL.Query().Get("version"), req.Start, req.Limit)
}

func GetAppVersionByID(req *Request) interface{} {
	return types.GetVersionByID(utils.Atoi(req.URL.Query().Get(":id"), 0))
}

func GetJob(req *Request) interface{} {
	return types.GetJob(utils.Atoi(req.URL.Query().Get(":id"), 0))
}

func GetJobs(req *Request) interface{} {
	status := utils.Atoi(req.URL.Query().Get("status"), -1)
	succ := utils.Atoi(req.URL.Query().Get("succ"), -1)
	name := req.URL.Query().Get("name")
	version := req.URL.Query().Get("version")
	return types.GetJobs(name, version, status, succ, req.Start, req.Limit)
}

func init() {
	RestAPIServer = pat.New()

	rs := map[string]map[string]func(*Request) interface{}{
		"POST": {
			"/app/:projectname/:version":           RegisterApplicationHandler,
			"/app/:app/:version/add":               AddContainerHandler,
			"/app/:app/:version/build":             BuildImageHandler,
			"/app/:app/:version/test":              TestImageHandler,
			"/app/:app/:version/deploy":            DeployApplicationHandler,
			"/app/:app/:version/update":            UpdateApplicationHandler,
			"/app/:app/:version/remove":            RemoveApplicationHandler,
			"/appversion/:projectname/:version":    RegisterApplicationHandler,
			"/appversion/:app/:version/add":        AddContainerHandler,
			"/appversion/:app/:version/build":      BuildImageHandler,
			"/appversion/:app/:version/test":       TestImageHandler,
			"/appversion/:app/:version/deploy":     DeployApplicationHandler,
			"/appversion/:app/:version/update":     UpdateApplicationHandler,
			"/appversion/:app/:version/remove":     RemoveApplicationHandler,
			"/appversion/:app/:version/subappyaml": AddSubAppYamlHandler,
			"/container/:cid/remove":               RemoveContainerHandler,
			"/resource/:app/mysql":                 NewMySQLInstanceHandler,
			"/resource/:app/syncdb":                SyncDBHandler,
			"/resource/:app/redis":                 NewRedisInstanceHandler,
			"/resource/:app/sentry":                NewSentryDSNHandler,
			"/resource/:app/influxdb":              NewInfluxdbHandler,
			"/resource/:app/remove":                RemoveResourceHandler,
		},
		"GET": {
			"/echo":                                EchoHandler,
			"/app":                                 GetAllApplications,
			"/app/:app":                            GetApplication,
			"/app/:app/branch":                     AppBranchHandler,
			"/app/:app/jobs":                       GetAppJobs,
			"/app/:app/containers":                 GetAppContainers,
			"/app/:app/versions":                   GetAppVersions,
			"/appversion/:app/:version":            GetAppVersion,
			"/appversion/:app/:version/jobs":       GetAppVersionJobs,
			"/appversion/:app/:version/containers": GetAppVersionContainers,
			"/appversion/:app/:version/subappyaml": ListSubAppYamlHandler,
			"/appversion/:id":                      GetAppVersionByID,
			"/host/:id":                            GetHostByID,
			"/hosts":                               GetAllHosts,
			"/container/:cid":                      GetContainerByCid,
			"/containers":                          GetContainers,
			"/jobs":                                GetJobs,
			"/job/:id":                             GetJob,
		},
		"PUT": {
			"/app/:app/branch": AppBranchHandler,
		},
	}

	for method, routes := range rs {
		for route, handler := range routes {
			RestAPIServer.Add(method, route, http.HandlerFunc(JSONWrapper(handler)))
		}
	}
}
