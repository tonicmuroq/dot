package resources

import (
	"../config"
	"fmt"
	"net/url"
)

func NewRedisInstance(appname string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/start/%s", config.Config.Redismgr, appname)
	return Post(u, url.Values{})
}

func ExpandRedisInstance(appname string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/expand/%s", config.Config.Redismgr, appname)
	return Post(u, url.Values{})
}
