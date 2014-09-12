package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v1"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

func EnsureDir(path string, owner, group int) error {
	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}
	return os.Chown(path, owner, group)
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
	t := time.Now().String()
	code := CreateSha1HexValue([]byte(app.Name + app.Version + t))

	v := url.Values{}
	v.Set("SysUid", config.Dba.Sysuid)
	v.Set("SysPwd", config.Dba.Syspwd)
	v.Set("businessCode", config.Dba.Bcode)
	v.Set("DbName", app.Name)
	v.Set("DbUid", app.Name)
	v.Set("DbPwd", code[:8])
	if r, err := http.DefaultClient.PostForm(config.Dba.Addr, v); err == nil {
		defer r.Body.Close()
		result, _ := ioutil.ReadAll(r.Body)
		var d map[string]string
		json.Unmarshal(result, &d)
		if d["Result"] == "0" {
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

func CreateSha1HexValue(data []byte) string {
	r := sha1.Sum(data)
	x := make([]byte, len(r))
	for index, d := range r {
		x[index] = d
	}
	return hex.EncodeToString(x)
}

// 把src加上dst前缀整个copy
func CopyFiles(dst, src string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			e := os.Mkdir(dst+path, info.Mode())
			return e
		} else {
			d, e := os.Create(dst + path)
			defer d.Close()
			if e != nil {
				return e
			}

			f, e := os.Open(path)
			defer f.Close()
			if e != nil {
				return e
			}

			io.Copy(d, f)
		}
		return err
	})
}
