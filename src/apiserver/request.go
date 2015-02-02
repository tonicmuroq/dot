package apiserver

import (
	"net/http"

	"utils"
)

type Request struct {
	http.Request
	Start int
	Limit int
	User  string
}

// parse start, limit for data
func (r *Request) Init() {
	r.ParseForm()
	r.Start = utils.Atoi(r.Form.Get("start"), 0)
	r.Limit = utils.Atoi(r.Form.Get("limit"), 20)
	// 先删掉这个, 暂时没有接
	// r.User = r.Header.Get("NBE-User")
	r.User = "NBEBot"
}

func NewRequest(r *http.Request) *Request {
	req := &Request{*r, 0, 20, ""}
	req.Init()
	return req
}
