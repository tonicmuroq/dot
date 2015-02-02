package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"apiserver"
	"config"
	"dot"
	"types"
	. "utils"
)

var version = "Dot Version 0.3.0 (sub app support 2015.01.30)"

func main() {

	if len(os.Args) >= 2 && os.Args[1] == "version" {
		fmt.Println(version)
		os.Exit(0)
	}

	config.LoadConfig()
	types.LoadStore()

	go dot.LeviHub.CheckAlive()
	go dot.LeviHub.Run()

	http.Handle("/", apiserver.RestAPIServer)
	http.HandleFunc("/ws", dot.ServeWS)
	http.HandleFunc("/log", dot.ServeLogWS)

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
	dot.LeviHub.Close()
}
