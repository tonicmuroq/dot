package main

import (
	"log"
	"net/http"
)

func main() {
	go hub.CheckAlive()
	go taskqueue.Run()

	http.Handle("/", restserver)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal("err")
	}
}
