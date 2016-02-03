package forms

import (
	"fmt"
	"github.com/hlandau/degoutils/text"
	"net/http"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Given a struct s, set all the fields of it from the HTTP request as
// appropiate and add any errors to the errorSink. If, after doing this, there
// are errors in the errorSink, returns the errorSink. Note that this may
// happen even if no errors were added but the sink already contained errors.
func fromReq(s interface{}, req *http.Request, errorSink ErrorSink) error {
	sv := reflect.Indirect(reflect.ValueOf(s))
	st := sv.Type()

	numFields := sv.NumField()
	for i := 0; i < numFields; i++ {
		fv := sv.Field(i)
		ff := st.Field(i)

		fieldFromReq(fv, ff, req, errorSink)
	}

	if errorSink.Num() > 0 {
		return errorSink
	}

	return nil
}

// Given a struct field and a value which is that field for a specific instance
// of that struct, obtain the submitted value for the field from the given HTTP
// request and validate it.  If it is invalid, add one or more errors to the
// errorSink, otherwise set the value.
func fieldFromReq(fv reflect.Value, ff reflect.StructField, req *http.Request, errorSink ErrorSink) {
	finfo := GetFieldInfo(ff)

	if finfo.Name == "" {
		return
	}

	value := req.FormValue(finfo.Name)
	_, ok := req.Form[finfo.Name]
	if !ok && !finfo.Required {
		return
	}

	m := "Must match the required format."
	mi := "Must be an integer."
	mu := "Must be a non-negative integer."
	mf := "Must be a real number."
	mb := "Must be true or false."
	if finfo.FormatMessage != "" {
		m = finfo.FormatMessage
		mi = m
		mu = m
		mf = m
		mb = m
	}

	svalue := strings.TrimSpace(value)
	if finfo.Required && svalue == "" {
		errorSink.Add(ErrorInfo{
			FieldInfo: &finfo,
			Message:   m,
		})
		return
	}

	if finfo.MaxLength > 0 && len(value) > finfo.MaxLength {
		errorSink.Add(ErrorInfo{
			FieldInfo: &finfo,
			Message:   fmt.Sprintf("must not exceed %d characters", finfo.MaxLength),
		}) // XXX: bytes not characters
		return
	}

	if svalue != "" {
		if finfo.Pattern != "" {
			re := regexp.MustCompilePOSIX(finfo.Pattern) // TODO: cache this
			if !re.MatchString(value) {
				errorSink.Add(ErrorInfo{
					FieldInfo: &finfo,
					Message:   m,
				})
				return
			}
		}

		// Field type specific validation.
		switch finfo.Type {
		case "email":
			addr, err := mail.ParseAddress(value)
			if err != nil || addr.Name != "" {
				errorSink.Add(ErrorInfo{
					FieldInfo: &finfo,
					Message:   "must be a valid e. mail address",
				})
				return
			}

			value = addr.Address

		case "url":
			_, err := url.Parse(value)
			if err != nil {
				errorSink.Add(ErrorInfo{
					FieldInfo: &finfo,
					Message:   "must be a valid URL",
				})
				return
			}

			// TODO: more types.
		}
	}

	switch ff.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, bsz(ff.Type.Kind()))
		if err != nil {
			errorSink.Add(ErrorInfo{
				FieldInfo: &finfo,
				Message:   mi,
			})
			return
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, bsz(ff.Type.Kind()))
		if err != nil {
			errorSink.Add(ErrorInfo{
				FieldInfo: &finfo,
				Message:   mu,
			})
			return
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(value, bsz(ff.Type.Kind()))
		if err != nil {
			errorSink.Add(ErrorInfo{
				FieldInfo: &finfo,
				Message:   mf,
			})
			return
		}
		fv.SetFloat(n)
	case reflect.Bool:
		b, ok := text.ParseBoolForgiving(value)
		if !ok {
			errorSink.Add(ErrorInfo{
				FieldInfo: &finfo,
				Message:   mb,
			})
			return
		}
		fv.SetBool(b)
	case reflect.String:
		fv.SetString(value)
	default:
		panic(fmt.Sprintf("forms: don't know how to assign type %T", ff.Type))
	}
}

func bsz(k reflect.Kind) int {
	switch k {
	case reflect.Int8, reflect.Uint8:
		return 8
	case reflect.Int16, reflect.Uint16:
		return 16
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		return 32
	case reflect.Int64, reflect.Uint64, reflect.Float64:
		return 64
	default:
		return 32
	}
}
