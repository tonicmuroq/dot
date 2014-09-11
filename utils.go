package main

import (
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strconv"
)

func JSONDecode(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

func JSONEncode(v interface{}) (string, error) {
	r, err := json.Marshal(v)
	return string(r), err
}

func YAMLDecode(data string, v interface{}) error {
	return yaml.Unmarshal([]byte(data), v)
}

func YAMLEncode(v interface{}) (string, error) {
	r, err := yaml.Marshal(v)
	return string(r), err
}

func GetUid(username string) (string, error) {
	user, err := user.Lookup(username)
	if err != nil {
		return "", err
	}
	return user.Uid, nil
}

func GetGid(username string) (string, error) {
	user, err := user.Lookup(username)
	if err != nil {
		return "", err
	}
	return user.Gid, nil
}

func EnsureDir(path string, owner, group int) error {
	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}
	return os.Chown(path, owner, group)
}

func EnsureFile(path string, owner, group int, content []byte) error {
	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		return nil
	}
	file.Write(content)
	os.Chmod(path, 0755)
	os.Chown(path, owner, group)
	return nil
}

func EnsureDirAbsent(path string) error {
	return os.RemoveAll(path)
}

func EnsureFileAbsent(path string) error {
	return os.Remove(path)
}

func CreateDatabase(app *Application) (map[string]interface{}, error) {
	// TODO 接入数据库
	// businessCode := app.Name
	// dbName := app.Name
	// dbUid := app.Name
	// dbPwd := "123"
	v := url.Values{}
	v.Set("SysUid", config.Dba.Sysuid)
	v.Set("SysPwd", config.Dba.Syspwd)
	v.Set("businessCode", config.Dba.Bcode)
	v.Set("DbName", app.Name)
	v.Set("DbUid", app.Name)
	v.Set("DbPwd", "xxxxxx")
	if r, err := http.DefaultClient.PostForm(config.Dba.Addr, v); err == nil {
		defer r.Body.Close()
		result, _ := ioutil.ReadAll(r.Body)
		var d map[string]string
		json.Unmarshal(result, &d)
		if d["Result"] == "1" {
			return nil, errors.New("create mysql failed")
		}
		ret := make(map[string]interface{})
		ret["username"] = d["DbUser"]
		ret["password"] = d["DbPwd"]
		ret["host"] = d["IPAddress"]
		ret["port"], _ = strconv.Atoi(d["Port"])
		ret["db"] = d["DbName"]
		return ret, nil
	} else {
		return nil, err
	}
}

func CreateRedis(app *Application) map[string]interface{} {
	// TODO 接入redis
	r := make(map[string]interface{})
	r["host"] = "localhost"
	r["port"] = 6379
	return r
}
