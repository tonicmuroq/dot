package resources

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

func Post(addr string, form url.Values) (map[string]string, error) {
	r, err := http.PostForm(addr, form)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var data map[string]string
	err := json.Unmarshal(content, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
