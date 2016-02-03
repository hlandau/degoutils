package forms

import (
	"net/http"
	"net/url"
	"testing"
)

type exampleForm struct {
	Email    string `form:"email,required,255!" fmsg:"You must enter a valid e. mail address."`
	Password string `form:"password"`
}

func TestForm(t *testing.T) {
	var errs Errors

	req := &http.Request{
		Method: "GET",
		Form: url.Values{
			"Email":    []string{"john.smith@example.com"},
			"Password": []string{"blah"},
		},
	}

	var f exampleForm
	err := FromReq(&f, req, &errs)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	t.Logf("%#v", &f)
}
