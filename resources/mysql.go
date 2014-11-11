package resources

import (
	"../config"
	"../utils"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
	"net/url"
	"strconv"
	"time"
)

var (
	CreateError = errors.New("Create Database error.")
	GrantError  = errors.New("Grant error.")
)

func NewMySQLInstance(appname string) (map[string]interface{}, error) {
	var err error
	db := orm.NewOrm()
	db.Using(config.Config.Dbmgr.Name)

	password := utils.CreateSha1HexValue([]byte(appname + time.Now().String()))[:8]
	_, err = db.Raw(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", appname)).Exec()
	if err != nil {
		return nil, CreateError
	}

	_, err = db.Raw(fmt.Sprintf("GRANT DROP, CREATE, ALTER, SELECT, INSERT, "+
		"UPDATE, DELETE ON `%s`.* TO '%s'@'%%' IDENTIFIED BY '%s'", appname, appname, password)).Exec()
	if err != nil {
		return nil, GrantError
	}

	return map[string]interface{}{
		"username": appname,
		"password": password,
		"host":     "10.1.201.58", // FIXME ...
		"db":       appname,
		"port":     3306,
	}, nil
}

func NewMySQLInstance2(appname string) (map[string]interface{}, error) {
	password := utils.CreateSha1HexValue([]byte(appname + time.Now().String()))
	form := url.Values{
		"SysUid":       []string{config.Config.Dba.Sysuid},
		"SysPwd":       []string{config.Config.Dba.Syspwd},
		"businessCode": []string{config.Config.Dba.Bcode},
		"DbName":       []string{appname},
		"DbUid":        []string{appname},
		"DbPwd":        []string{password[:8]},
	}

	ret, err := Post(config.Config.Dba.Addr, form)
	if err != nil {
		return nil, err
	}
	if ret["Result"] == "0" {
		return nil, errors.New("Create MySQL failed")
	}
	port, _ := strconv.Atoi(ret["Port"])
	conn := map[string]interface{}{
		"username": ret["DbUser"],
		"password": ret["DbPwd"],
		"host":     ret["IPAddress"],
		"db":       ret["DbName"],
		"port":     port,
	}
	return conn, nil
}
