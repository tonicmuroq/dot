package resources

import (
	"../config"
	"fmt"
	"net/url"
	"path"
	"strconv"
)

func NewRedisInstance(appname string) (map[string]interface{}, error) {
	u := path.Join(config.Config.Redismgr, fmt.Sprintf("/start/%s", appname))
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
	u := path.Join(config.Config.Redismgr, fmt.Sprintf("/expand/%s", appname))
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
