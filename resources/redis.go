package resources

import (
	"../config"
	"fmt"
	"net/url"
	"strconv"
)

func NewRedisInstance(appname string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/start/%s", config.Config.Redismgr, appname)
	ret, err := Post(u, url.Values{})
	if err != nil {
		return nil, err
	}
	port, _ := strconv.Atoi(ret["port"])
	return map[string]interface{}{
		"host": ret["host"],
		"port": port,
	}, nil
}

func ExpandRedisInstance(appname string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/expand/%s", config.Config.Redismgr, appname)
	ret, err := Post(u, url.Values{})
	if err != nil {
		return nil, err
	}
	port, _ := strconv.Atoi(ret["port"])
	return map[string]interface{}{
		"host": ret["host"],
		"port": port,
	}, nil
}
