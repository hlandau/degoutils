package unixhttp

import (
	"net"
	"net/http"
)

func ListenAndServe(s *http.Server, path string) error {
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}
