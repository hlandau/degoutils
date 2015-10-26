package net

import "net/http"
import "fmt"

func Require200(res *http.Response, err error) (*http.Response, error) {
	if res != nil && err != nil && res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("non-200 status code: %s", res.Status)
	}

	return res, err
}
