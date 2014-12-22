package config

import (
	. "../utils"
	"flag"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v1"
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
	CpuShare  int
	CpuSet    string
}

type NginxConfig struct {
	Template     string
	Staticdir    string
	Staticsrcdir string
	Conf         string
	Port         int
}

type DbaConfig struct {
	Sysuid string
	Syspwd string
	Bcode  string
	Addr   string
}

type DotConfig struct {
	Bind       string
	Pidfile    string
	Masteraddr string
	Redismgr   string
	Minport    int
	Maxport    int
	DNSSuffix  string `yaml:"dns_suffix"`

	Db    DbConfig
	Dbmgr DbConfig
	Etcd  EtcdConfig
	Task  TaskConfig
	Nginx NginxConfig
	Dba   DbaConfig
}

var Config = DotConfig{}

func LoadConfig() {
	var configPath string
	flag.BoolVar(&Logger.Mode, "DEBUG", false, "enable debug")
	flag.StringVar(&configPath, "c", "dot.yaml", "config file")
	flag.Parse()

	if _, err := os.Stat(configPath); err != nil {
		Logger.Assert(err, "config file invaild")
	}

	b, err := ioutil.ReadFile(configPath)
	if err != nil {
		Logger.Assert(err, "Read config file failed")
	}

	if err := yaml.Unmarshal(b, &Config); err != nil {
		Logger.Assert(err, "Load config file failed")
	}
}
