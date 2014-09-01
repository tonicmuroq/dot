package main

import (
	"github.com/bmizerany/pat"
	"io"
	"net/http"
)

var restserver *pat.PatternServeMux

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
	b, _ := JSONEncode(r)
	io.WriteString(w, b)
}

func DeployApplicationHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	app := GetApplicationByNameAndVersion(name, version)

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app == nil {
		r["r"] = 1
		r["msg"] = "no such app"
		// TODO now just for testing
		task := []byte(name + version)
		taskqueue.AddTask(&task)
	} else {
		// deploy app
	}
	b, _ := JSONEncode(r)
	io.WriteString(w, b)
}

func init() {
	restserver = pat.New()
	restserver.Get("/hello/:name", http.HandlerFunc(HelloServer))
	restserver.Post("/app/:app/version/:version", http.HandlerFunc(RegisterApplicationHandler))
	restserver.Post("/app/:app/version/:version/deploy", http.HandlerFunc(DeployApplicationHandler))
}
