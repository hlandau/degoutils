package forms

import (
	"bytes"
	"fmt"
	"github.com/flosch/pongo2"
	"html"
	"net/http"
	"reflect"
	"strings"
)

// Field layout functions with additional surrounding divs for layout support.
func GridHTML(fi *FieldInfo, value string, req *http.Request, buf *bytes.Buffer) {
	if fi.Type == "hidden" {
		InputHTML(fi, value, req, buf)
		return
	}

	fmt.Fprintf(buf, `<div class="form-field"><label for="%s">%s</label>`, html.EscapeString(fi.ID), html.EscapeString(fi.Label))
	InputHTML(fi, value, req, buf)
	buf.WriteString(`</div>`)
}

// Field layout function for the field itself only, without any surrounding
// divs, labels, etc.
func InputHTML(fi *FieldInfo, value string, req *http.Request, buf *bytes.Buffer) {
	switch fi.Type {
	case "select":
		buf.WriteString(`<select`)
		generalHTML(fi, buf)
		if fi.Size != 0 {
			fmt.Fprintf(buf, ` size="%d"`, fi.Size)
		}
		buf.WriteString(`>`)

		f, ok := SelectValueFuncs[fi.ValueSet]
		if ok {
			vs := f(fi, req)
			curGroup := ""
			for i := range vs {
				if vs[i].Group != curGroup {
					if curGroup != "" {
						buf.WriteString(`</optgroup>`)
					}
					if vs[i].Group != "" {
						fmt.Fprintf(buf, `<optgroup title="%s">`, html.EscapeString(vs[i].Group))
					}
					curGroup = vs[i].Group
				}

				fmt.Fprintf(buf, `<option value="%s"`, html.EscapeString(vs[i].Value))
				if vs[i].Disabled {
					buf.WriteString(` disabled=""`)
				}
				if vs[i].Value == value {
					buf.WriteString(` selected=""`)
				}
				fmt.Fprintf(buf, `>%s</option>`, html.EscapeString(vs[i].Title))
			}
		}

		buf.WriteString(`</select>`)

	case "textarea":
		buf.WriteString(`<textarea`)
		generalHTML(fi, buf)
		if fi.Size != 0 {
			fmt.Fprintf(buf, ` cols="%d"`, fi.Size)
		}
		if fi.VSize != 0 {
			fmt.Fprintf(buf, ` rows="%d"`, fi.VSize)
		}
		fmt.Fprintf(buf, `>%s</textarea>`, html.EscapeString(value))

	default:
		fmt.Fprintf(buf, `<input type="%s" value="%s"`, html.EscapeString(fi.Type), html.EscapeString(value))
		generalHTML(fi, buf)
		if fi.Size != 0 {
			fmt.Fprintf(buf, ` size="%d"`, fi.Size)
		}
		buf.WriteString(`/>`)
	}
}

func generalHTML(fi *FieldInfo, buf *bytes.Buffer) {
	if fi.Name != "" {
		fmt.Fprintf(buf, ` name="%s"`, html.EscapeString(fi.Name))
	}

	if fi.ID != "" {
		fmt.Fprintf(buf, ` id="%s"`, html.EscapeString(fi.ID))
	}

	if len(fi.Classes) > 0 {
		fmt.Fprintf(buf, ` class="%s"`, html.EscapeString(strings.Join(fi.Classes, " ")))
	}

	if fi.Required {
		buf.WriteString(` required=""`)
	}
	if fi.Autofocus {
		buf.WriteString(` autofocus=""`)
	}
	if fi.Pattern != "" {
		fmt.Fprintf(buf, ` pattern="%s"`, html.EscapeString(fi.Pattern))
	}
	if fi.FormatMessage != "" {
		fmt.Fprintf(buf, ` title="%s" x-moz-errormessage="%s"`, html.EscapeString(fi.FormatMessage), html.EscapeString(fi.FormatMessage))
	}
	if fi.Placeholder != "" && fi.Type != "hidden" {
		fmt.Fprintf(buf, ` placeholder="%s"`, html.EscapeString(fi.Placeholder))
	}
	if fi.MaxLength != 0 {
		fmt.Fprintf(buf, ` maxlength="%d"`, fi.MaxLength)
	}
}

type HTMLFunc func(fi *FieldInfo, value string, req *http.Request, buf *bytes.Buffer)

// Generates HTML (as a pongo2 safe value) for the given form. The given
// function is used for field layout.
func FormHTML(s interface{}, req *http.Request, htmlFunc HTMLFunc) *pongo2.Value {
	buf := bytes.Buffer{}
	sv := reflect.Indirect(reflect.ValueOf(s))
	st := sv.Type()

	numFields := sv.NumField()
	for i := 0; i < numFields; i++ {
		fv := sv.Field(i)
		ff := st.Field(i)
		finfo := GetFieldInfo(ff)

		value := fmt.Sprintf("%v", fv.Interface())
		htmlFunc(&finfo, value, req, &buf)
	}

	return pongo2.AsSafeValue(buf.String())
}
