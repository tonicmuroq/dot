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

type DotConfig struct {
	Bind    string
	PidFile string

	Db   DbConfig
	Etcd EtcdConfig
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
	logger.Debug(config)
}

func init() {
	LoadConfig()
}
