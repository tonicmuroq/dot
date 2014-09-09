package main

import (
	"flag"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"os"
)

type DbConfig struct {
	Use  string
	Name string
	Url  string
}

type EtcdConfig struct {
	Sync     bool
	Machines []string
}

type TaskConfig struct {
	Dispatch  int
	Queuesize int
	Memory    int
	Cpus      int
}

type NginxConfig struct {
	Template      string
	Conf          string
	Levinginxport int
}

type DotConfig struct {
	Bind    string
	Pidfile string

	Db    DbConfig
	Etcd  EtcdConfig
	Task  TaskConfig
	Nginx NginxConfig
}

var config = DotConfig{}

func LoadConfig() {
	var configPath string
	flag.BoolVar(&logger.Mode, "DEBUG", false, "enable debug")
	flag.StringVar(&configPath, "c", "dot.yaml", "config file")
	flag.Parse()

	if _, err := os.Stat(configPath); err != nil {
		logger.Assert(err, "config file invaild")
	}

	b, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Assert(err, "Read config file failed")
	}

	if err := yaml.Unmarshal(b, &config); err != nil {
		logger.Assert(err, "Load config file failed")
	}
}

func init() {
	LoadConfig()
}
