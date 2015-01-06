package resources

import (
	"../config"
	"../utils"
	"database/sql"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
	"time"
)

var (
	CreateError = errors.New("Create Database error.")
	GrantError  = errors.New("Grant error.")
)

func NewMySQLInstance(dbname, username string) (map[string]interface{}, error) {
	var err error
	db := orm.NewOrm()
	db.Using(config.Config.Dbmgr.Name)

	password := utils.CreateSha1HexValue([]byte(username + time.Now().String()))[:8]
	_, err = db.Raw(fmt.Sprintf("CREATE DATABASE `%s`", dbname)).Exec()
	if err != nil {
		return nil, CreateError
	}

	_, err = db.Raw(fmt.Sprintf("GRANT DROP, CREATE, ALTER, SELECT, INSERT, "+
		"UPDATE, DELETE ON `%s`.* TO '%s'@'%%' IDENTIFIED BY '%s'", username, username, password)).Exec()
	if err != nil {
		return nil, GrantError
	}

	return map[string]interface{}{
		"username": username,
		"password": password,
		"host":     "10.1.201.58", // FIXME ...
		"db":       dbname,
		"port":     3306,
	}, nil
}

func SyncSchema(dsn, schema string) error {
	conn, err := sql.Open("mysql", dsn)
	defer conn.Close()
	if err != nil {
		return err
	}
	_, err = conn.Exec(schema)
	if err != nil {
		return err
	}
	return nil
}
