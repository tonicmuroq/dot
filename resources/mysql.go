package resources

import (
	"../config"
	"../utils"
	"errors"
	"net/url"
	"strconv"
	"time"
)

func NewMySQLInstance(appname string) (map[string]interface{}, error) {
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
