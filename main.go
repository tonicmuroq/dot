package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"./config"
	"./models"
	. "./utils"
)

var version = "Dot Version 0.2.0 (fix pod name 2014.01.16)"

func main() {

	if len(os.Args) >= 2 && os.Args[1] == "version" {
		fmt.Println(version)
		os.Exit(0)
	}

	config.LoadConfig()
	models.LoadStore()

	go hub.CheckAlive()
	go hub.Run()

	http.Handle("/", RestServer)
	http.HandleFunc("/ws", ServeWs)
	http.HandleFunc("/log", ServeLogWs)

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
