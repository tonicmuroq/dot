package main

import (
	"github.com/bmizerany/pat"
	"io"
	"log"
	"net/http"
)

func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hellp, "+req.URL.Query().Get(":name"))
}

func AppnameAndVersion(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, req.URL.Query().Get(":app")+"    "+req.URL.Query().Get(":version"))
}

func main() {
	m := pat.New()
	m.Get("/hello/:name", http.HandlerFunc(HelloServer))
	m.Get("/app/:app/version/:version", http.HandlerFunc(AppnameAndVersion))

	go hub.checkAlive()

	http.Handle("/", m)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal("err")
	}
}
