package main

import (
	"./config"
	"./models"
	. "./utils"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config.LoadConfig()
	models.LoadStore()

	go hub.CheckAlive()
	go hub.Run()

	http.Handle("/", restServer)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(config.Config.Bind, nil)
	if err != nil {
		Logger.Assert(err, "http")
	}

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, os.Interrupt)
		signal.Notify(sc, syscall.SIGTERM)
		signal.Notify(sc, syscall.SIGHUP)
		Logger.Info("Got <-", <-sc)
		hub.Close()
		os.Exit(0)
	}()
}
