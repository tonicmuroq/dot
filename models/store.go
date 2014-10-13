package models

import (
	"../config"
	"github.com/astaxie/beego/orm"
	"github.com/coreos/go-etcd/etcd"
	_ "github.com/go-sql-driver/mysql"
	"sync"
)

var db orm.Ormer
var etcdClient *etcd.Client
var portMutex sync.Mutex

func LoadStore() {
	// mysql
	orm.RegisterDataBase(config.Config.Db.Name, config.Config.Db.Use, config.Config.Db.Url, 30)
	orm.RegisterModel(new(Application), new(User), new(Host), new(Container), new(HostPort))
	orm.RunSyncdb(config.Config.Db.Name, false, false)
	db = orm.NewOrm()

	// etcd
	etcdClient = etcd.NewClient(config.Config.Etcd.Machines)
	if config.Config.Etcd.Sync {
		etcdClient.SyncCluster()
	}

	// Mutex
	portMutex = sync.Mutex{}
}
