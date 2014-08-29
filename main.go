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

func main() {
	m := pat.New()
	m.Get("/hello/:name", http.HandlerFunc(HelloServer))
	m.Post("/app/:app/version/:version", http.HandlerFunc(RegisterApplicationHandler))

	go hub.checkAlive()

	http.Handle("/", m)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal("err")
	}
}
