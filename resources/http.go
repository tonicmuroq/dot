package resources

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

func Post(addr string, form url.Values) (map[string]string, error) {
	if r, err := http.DefaultClient.PostForm(addr, form); err == nil {
		defer r.Body.Close()
		if content, err := ioutil.ReadAll(r.Body); err == nil {
			var data map[string]string
			json.Unmarshal(content, &data)
			return data, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}
