package main

import (
	"encoding/json"
	"github.com/bmizerany/pat"
	"io"
	"net/http"
)

var restServer *pat.PatternServeMux

type JsonTmpl map[string]interface{}

func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hello, ["+req.URL.Query().Get(":name")+"]")
}

func RegisterApplicationHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	projectname := req.URL.Query().Get(":app")
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

	app := GetApplicationByNameAndVersion(name, version)
	host := GetHostByIP(ip)

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app == nil || host == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		task := AddContainerTask(app, host, false)
		logger.Debug("add container task ", task)
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
		logger.Debug("build image task ", task)
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

	r := JsonTmpl{"r": 0, "msg": "ok"}
	app := GetApplicationByNameAndVersion(name, version)
	hosts := GetHostsByIPs(ips)
	if app == nil {
		r["r"] = 1
		r["msg"] = "no such app"
	} else {
		if err := DeployApplicationHelper(app, hosts, false); err != nil {
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

func init() {
	restServer = pat.New()
	restServer.Get("/hello/:name", http.HandlerFunc(HelloServer))
	restServer.Post("/app/:app/:version", http.HandlerFunc(RegisterApplicationHandler))
	restServer.Post("/app/:app/:version/add", http.HandlerFunc(AddContainerHandler))
	restServer.Post("/app/:app/:version/build", http.HandlerFunc(BuildImageHandler))
	restServer.Post("/app/:app/:version/deploy", http.HandlerFunc(DeployApplicationHandler))
	restServer.Post("/app/:app/:version/remove", http.HandlerFunc(RemoveApplicationHandler))
}
