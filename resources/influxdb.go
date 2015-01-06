package resources

import (
	"../config"
	"../utils"
	"fmt"
	"github.com/influxdb/influxdb/client"
	"time"
)

func NewInfluxdb(appname string) (map[string]interface{}, error) {
	password := utils.CreateSha1HexValue([]byte(appname + time.Now().String()))[:8]
	client, err := client.NewClient(&client.ClientConfig{
		Host:     fmt.Sprintf("%s:%s", config.Config.Influxdb.Host, config.Config.Influxdb.Port),
		Username: config.Config.Influxdb.Username,
		Password: config.Config.Influxdb.Password,
		IsSecure: false,
		IsUDP:    false,
	})
	if err != nil {
		return nil, err
	}
	err = client.CreateDatabase(appname)
	if err != nil {
		return nil, err
	}
	err = client.CreateDatabaseUser(appname, appname, password)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"username": appname,
		"password": password,
		"host":     config.Config.Influxdb.Host,
		"port":     config.Config.Influxdb.Port,
		"db":       appname,
	}, nil
}
