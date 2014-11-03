package resources

import (
	"time"
)

func NewRedisInstance(appname string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"host": "10.1.201.88",
		"port": time.Now().Nanosecond()%13 + 2000,
	}, nil
}
