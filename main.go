package main

import (
	"net/http"
)

func main() {
	go hub.CheckAlive()
	go taskhub.Run()

	http.Handle("/", restServer)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(config.Bind, nil)
	if err != nil {
		logger.Assert(err, "http")
	}
}
