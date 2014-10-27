package main

import (
	"./models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bmizerany/pat"
)

var restServer *pat.PatternServeMux

type JsonTmpl map[string]interface{}

func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hello, ["+req.URL.Query().Get(":name")+"]")
}

func RegisterApplicationHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	projectname := req.URL.Query().Get(":projectname")
	version := req.URL.Query().Get(":version")
	group := req.Form.Get("group")
	appyaml := req.Form.Get("appyaml")
	configyaml := req.Form.Get("configyaml")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app := models.NewApplication(projectname, version, group, appyaml, configyaml); app == nil {
		r["r"] = 1
		r["msg"] = "error"
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func AddContainerHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")

	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else if appyaml, err := app.GetAppYaml(); err != nil || (appyaml.Port == 0 && !appyaml.Daemon) {
		r["r"] = 1
		r["msg"] = "app port is 0 or no daemon"
	} else {
		task := models.AddContainerTask(app, host)
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_id"] = task.Id
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func BuildImageHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	// 暂时没有monitor, 那么人肉指定host吧
	ip := req.Form.Get("host")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		base := req.Form.Get("base")
		task := models.BuildImageTask(app, base)
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_id"] = task.Id
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func TestImageHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	// 暂时没有monitor, 那么人肉指定host吧
	ip := req.Form.Get("host")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		task := models.TestApplicationTask(app, host)
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_id"] = task.Id
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func DeployApplicationHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ips := req.Form["hosts"]

	r := JsonTmpl{"r": 0, "msg": "ok"}
	app := models.GetApplicationByNameAndVersion(name, version)
	hosts := models.GetHostsByIPs(ips)
	if app == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else if appyaml, err := app.GetAppYaml(); err != nil || (appyaml.Port == 0 && !appyaml.Daemon) {
		r["r"] = 1
		r["msg"] = "app port is 0 or no daemon"
	} else {
		if taskIds, err := DeployApplicationHelper(app, hosts); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_ids"] = taskIds
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func RemoveApplicationHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	ip := req.Form.Get("host")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	app := models.GetApplicationByNameAndVersion(name, version)
	host := models.GetHostByIP(ip)
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		if taskIds, err := RemoveApplicationFromHostHelper(app, host); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_ids"] = taskIds
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func UpdateApplicationHandler(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()

	name := req.URL.Query().Get(":app")
	fromVersion := req.URL.Query().Get(":version")

	ips := req.Form["hosts"]
	toVersion := req.Form.Get("to")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	fromApp := models.GetApplicationByNameAndVersion(name, fromVersion)
	toApp := models.GetApplicationByNameAndVersion(name, toVersion)
	hosts := models.GetHostsByIPs(ips)
	if fromApp == nil || toApp == nil {
		r["r"] = 1
		r["msg"] = fmt.Sprintf("no such app %s, %s", fromApp, toApp)
	} else {
		if taskIds, err := UpdateApplicationHelper(fromApp, toApp, hosts); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_ids"] = taskIds
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func RemoveContainerHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	cid := req.URL.Query().Get(":cid")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	container := models.GetContainerByCid(cid)
	if container == nil {
		r["r"] = 1
		r["msg"] = "no such container"
	} else {
		host := container.Host()
		task := models.RemoveContainerTask(container)
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
		} else {
			r["task_id"] = task.Id
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func init() {
	restServer = pat.New()
	restServer.Get("/hello/:name", http.HandlerFunc(HelloServer))
	restServer.Post("/app/:projectname/:version", http.HandlerFunc(RegisterApplicationHandler))
	restServer.Post("/app/:app/:version/add", http.HandlerFunc(AddContainerHandler))
	restServer.Post("/app/:app/:version/build", http.HandlerFunc(BuildImageHandler))
	restServer.Post("/app/:app/:version/test", http.HandlerFunc(TestImageHandler))
	restServer.Post("/app/:app/:version/deploy", http.HandlerFunc(DeployApplicationHandler))
	restServer.Post("/app/:app/:version/update", http.HandlerFunc(UpdateApplicationHandler))
	restServer.Post("/app/:app/:version/remove", http.HandlerFunc(RemoveApplicationHandler))

	restServer.Post("/container/:cid/remove", http.HandlerFunc(RemoveContainerHandler))
}
