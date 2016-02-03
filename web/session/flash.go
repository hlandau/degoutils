package session

import "net/http"
import "encoding/gob"

func init() {
	// Allow Flash and arrays thereof to be serialized.
	gob.Register([]Flash{})
}

// A message which is returned once from the session store and then erased
// automatically.
type Flash struct {
	// Should be one of "success", "error", "warning", "info"
	Severity string

	// Message to display.
	// May contain inline HTML, but should not contain block-level elements.
	Msg string
}

// Add a flash message to the session for the given request.
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

// Retrieve the list of flash messages for the session for the given request.
// The messages are deleted from the session automatically, so future calls to
// this function will return an empty slice.
func GetFlash(req *http.Request) []Flash {
	var f []Flash
	fi, ok := Get(req, "flash")
	if ok {
		if ff, ok := fi.([]Flash); ok {
			f = ff
		}
	}

	if len(f) > 0 {
		Set(req, "flash", nil)
	}

	return f
}
