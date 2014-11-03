package resources

import (
	"../config"
	"../utils"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
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

	if r, err := http.DefaultClient.PostForm(config.Config.Dba.Addr, form); err == nil {
		defer r.Body.Close()
		var d map[string]string
		result, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(result, &d)

		utils.Logger.Info("Return value from DBA: ", d)

		if d["Result"] == "0" {
			return nil, errors.New("Create mysql failed")
		}

		port, _ := strconv.Atoi(d["Port"])
		ret := map[string]interface{}{
			"username": d["DbUser"],
			"password": d["DbPwd"],
			"host":     d["IPAddress"],
			"db":       d["DbName"],
			"port":     port,
		}
		return ret, nil
	} else {
		utils.Logger.Info("create mysql error: ", err)
		return nil, err
	}
}
