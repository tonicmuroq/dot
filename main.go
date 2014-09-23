package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	go hub.CheckAlive()
	go hub.Run()

	http.Handle("/", restServer)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(config.Bind, nil)
	if err != nil {
		logger.Assert(err, "http")
	}

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, os.Interrupt)
		signal.Notify(sc, syscall.SIGTERM)
		signal.Notify(sc, syscall.SIGHUP)
		logger.Info("Got <-", <-sc)
		hub.Close()
		os.Exit(0)
	}()
}
