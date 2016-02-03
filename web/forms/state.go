package forms

import "net/http"

// Form state.
type State struct {
	Errors Errors

	f   interface{}
	req *http.Request
}

// Load form from request. Pass pointer to structure to fill.
func (s *State) FromReq(f interface{}, req *http.Request) {
	s.f = f
	s.req = req
	fromReq(f, req, &s.Errors)
}

func FromReq(f interface{}, req *http.Request) *State {
	s := &State{}
	s.FromReq(f, req)
	return s
}

// Returns true if the form is being submitted and has no errors.
func (s *State) CanSubmit() bool {
	return s.req.Method == "POST" && s.Errors.Num() == 0
}
