package types

import (
	"sync"

	"github.com/astaxie/beego/orm"
	"github.com/coreos/go-etcd/etcd"
	_ "github.com/go-sql-driver/mysql"

	"config"
)

var (
	db         orm.Ormer
	etcdClient *etcd.Client
	portMutex  sync.Mutex
)

func LoadStore() {
	// mysql
	orm.RegisterDataBase(config.Config.Db.Name, config.Config.Db.Use, config.Config.Db.Url, 30)
	orm.RegisterDataBase(config.Config.Dbmgr.Name, config.Config.Dbmgr.Use, config.Config.Dbmgr.Url, 30)

	orm.RegisterModel(new(Application), new(AppVersion), new(User), new(Host), new(Container), new(Port), new(Job))
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
