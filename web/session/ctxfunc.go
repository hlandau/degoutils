package session

import "net/http"

func Get(req *http.Request, k string) (interface{}, bool) {
	c := getContext(req)
	return c.GetSession(k)
}

func Set(req *http.Request, k string, v interface{}) error {
	c := getContext(req)
	return c.SetSession(k, v)
}

func Save(req *http.Request) {
	c := getContext(req)
	c.Save()
}

func Delete(req *http.Request, k string) {
	c := getContext(req)
	c.DeleteSession(k)
}

func Bump(req *http.Request) {
	c := getContext(req)
	c.BumpSession()
}
