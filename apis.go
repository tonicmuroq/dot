package main

import (
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
	appyaml := req.Form.Get("appyaml")
	configyaml := req.Form.Get("configyaml")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app := NewApplication(projectname, version, appyaml, configyaml); app == nil {
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
	daemon := req.Form.Get("daemon")

	app := GetApplicationByNameAndVersion(name, version)
	host := GetHostByIP(ip)

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else if appyaml, err := app.GetAppYaml(); err != nil || (appyaml.Port == 0 && !appyaml.Daemon) {
		r["r"] = 1
		r["msg"] = "app port is 0 or no daemon"
	} else {
		task := AddContainerTask(app, host, daemon == "true")
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
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
	app := GetApplicationByNameAndVersion(name, version)
	host := GetHostByIP(ip)
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		group := req.Form.Get("group")
		base := req.Form.Get("base")
		task := BuildImageTask(app, group, base)
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
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
	app := GetApplicationByNameAndVersion(name, version)
	host := GetHostByIP(ip)
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		task := TestApplicationTask(app, host)
		if err := hub.Dispatch(host.IP, task); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
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
	daemon := req.Form.Get("daemon")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	app := GetApplicationByNameAndVersion(name, version)
	hosts := GetHostsByIPs(ips)
	if app == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		if err := DeployApplicationHelper(app, hosts, daemon == "true"); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
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
	app := GetApplicationByNameAndVersion(name, version)
	host := GetHostByIP(ip)
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		if err := RemoveApplicationFromHostHelper(app, host); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
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
	fromApp := GetApplicationByNameAndVersion(name, fromVersion)
	toApp := GetApplicationByNameAndVersion(name, toVersion)
	hosts := GetHostsByIPs(ips)
	if fromApp == nil || toApp == nil {
		r["r"] = 1
		r["msg"] = fmt.Sprintf("no such app %s, %s", fromApp, toApp)
	} else {
		if err := UpdateApplicationHelper(fromApp, toApp, hosts); err != nil {
			r["r"] = 1
			r["msg"] = err.Error()
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
}
