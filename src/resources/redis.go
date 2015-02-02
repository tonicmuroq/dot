package resources

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"config"
)

func NewRedisInstance(appname string) (map[string]interface{}, error) {
	rs := strings.Split(config.Config.Redismgr, ":")
	port, _ := strconv.Atoi(rs[1])
	return map[string]interface{}{"host": rs[0], "port": port}, nil
}

func ExpandRedisInstance(appname string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/expand/%s", config.Config.Redismgr, appname)
	return Post(u, url.Values{})
}
