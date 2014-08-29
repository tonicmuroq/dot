package main

import (
	"encoding/json"
	"io"
	"net/http"
)

type JsonTmpl map[string]interface{}

func RegisterApplicationHandler(w *http.ResponseWriter, req *http.Request) {
	projectname := req.URL.Query().Get(":app")
	version := req.URL.Query().Get(":version")
	appyaml := req.Form.Get("appyaml")
	configyaml := req.Form.Get("configyaml")

	r := JsonTmpl{"r": 0, "msg": "ok"}
	if app := NewApplication(projectname, version, appyaml, configyaml); app == nil {
		r["r"] = 1
		r["msg"] = "error"
	}
	b, _ := json.Marshal(r)
	io.WriteString(w, string(b))
}
