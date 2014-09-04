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

func DeployApplicationHandler(w http.ResponseWriter, req *http.Request) {
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
		if err := hub.Dispatch(host.Ip, task); err != nil {
			r["r"] = 0
			r["msg"] = err.Error()
		}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func FinishDispatchHandler(w http.ResponseWriter, req *http.Request) {
	r := JsonTmpl{"r": 0, "msg": "ok"}
	encoder := json.NewEncoder(w)
	encoder.Encode(r)
}

func init() {
	restServer = pat.New()
	restServer.Get("/hello/:name", http.HandlerFunc(HelloServer))
	restServer.Post("/app/:app/version/:version", http.HandlerFunc(RegisterApplicationHandler))
	restServer.Post("/app/:app/version/:version/deploy", http.HandlerFunc(DeployApplicationHandler))
	restServer.Get("/finish", http.HandlerFunc(FinishDispatchHandler))
}
