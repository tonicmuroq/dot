package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"./config"
	"./models"
	. "./utils"
)

func main() {
	config.LoadConfig()
	models.LoadStore()

	go hub.CheckAlive()
	go hub.Run()

	http.Handle("/", RestServer)
	http.HandleFunc("/ws", ServeWs)

	err := http.ListenAndServe(config.Config.Bind, nil)
	if err != nil {
		Logger.Assert(err, "http")
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	signal.Notify(sc, syscall.SIGTERM)
	signal.Notify(sc, syscall.SIGHUP)
	signal.Notify(sc, syscall.SIGKILL)
	signal.Notify(sc, syscall.SIGQUIT)
	Logger.Info("Got <-", <-sc)
	hub.Close()
}
