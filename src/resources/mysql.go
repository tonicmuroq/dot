package resources

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"

	"config"
	"utils"
)

var (
	CreateError = errors.New("Create Database error.")
	GrantError  = errors.New("Grant error.")
)

func NewMySQLInstance(dbname, username string) (map[string]interface{}, error) {
	var err error
	db := orm.NewOrm()
	db.Using(config.Config.Dbmgr.Name)

	password := utils.RandomString(8)
	_, err = db.Raw(fmt.Sprintf("CREATE DATABASE `%s`", dbname)).Exec()
	if err != nil {
		utils.Logger.Info("create error, ", err)
		return nil, CreateError
	}

	_, err = db.Raw(fmt.Sprintf("GRANT DROP, CREATE, ALTER, SELECT, INSERT, "+
		"UPDATE, DELETE ON `%s`.* TO '%s'@'%%' IDENTIFIED BY '%s'", username, username, password)).Exec()
	if err != nil {
		utils.Logger.Info("grant error, ", err)
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

	lines := strings.Split(schema, "\n")
	statements := []string{}
	for _, line := range lines {
		k := strings.TrimSpace(line)
		if k == "" || strings.HasPrefix(k, "/*") || strings.HasPrefix(k, "--") {
			continue
		}
		statements = append(statements, line)
		if strings.HasSuffix(line, ";") {
			statement := strings.Join(statements, "")
			_, err := conn.Exec(statement)
			if err != nil {
				return err
			}
			statements = []string{}
		}
	}
	return nil
}
