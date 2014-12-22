package resources

import (
	"../config"
	"../models"
	"fmt"
	"net/url"
)

func NewSentryDSN(appname, platform string) (map[string]interface{}, error) {
	app := models.GetApplication(appname)
	if app == nil {
		return nil, fmt.Errorf("No application %s found", appname)
	}
	data := url.Values{
		"team":     []string{app.Namespace},
		"platform": []string{platform},
		"project":  []string{appname},
	}
	u := fmt.Sprintf("%s/register_dsn", config.Config.Sentrymgr)
	return Post(u, data)
}
