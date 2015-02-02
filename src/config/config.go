package config

import (
	"flag"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v1"

	. "utils"
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

	UpstreamTemplate string `yaml:"upstream_template"`
	LocalUpDir       string `yaml:"local_up_dir"`
	RemoteUpDir      string `yaml:"remote_up_dir"`

	ServerTemplate  string `yaml:"server_template"`
	LocalServerDir  string `yaml:"local_server_dir"`
	RemoteServerDir string `yaml:"remote_server_dir"`
}

type InfluxdbConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type DotConfig struct {
	Bind       string
	Pidfile    string
	Masteraddr string
	Redismgr   string
	Sentrymgr  string
	Minport    int
	Maxport    int
	DNSSuffix  string `yaml:"dns_suffix"`
	PodName    string `yaml:"podname"`

	Db       DbConfig
	Dbmgr    DbConfig
	Etcd     EtcdConfig
	Task     TaskConfig
	Nginx    NginxConfig
	Influxdb InfluxdbConfig
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
