package session

import "net/http"
import "encoding/gob"

func init() {
	gob.Register([]Flash{})
}

type Flash struct {
	Severity string
	Msg      string
}

func AddFlash(req *http.Request, flash Flash) {
	var f []Flash
	fi, ok := Get(req, "flash")
	if ok {
		if ff, ok := fi.([]Flash); ok {
			f = ff
		}
	}

	f = append(f, flash)
	Set(req, "flash", f)
}

func GetFlash(req *http.Request) []Flash {
	var f []Flash
	fi, ok := Get(req, "flash")
	if ok {
		if ff, ok := fi.([]Flash); ok {
			f = ff
		}
	}

	if len(f) > 0 {
		Set(req, "flash", []Flash{})
	}

	return f
}
