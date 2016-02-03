package forms

import (
	"github.com/serenize/snaker"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Information about a form field.
type FieldInfo struct {
	FName         string   // struct field name
	Name          string   // <input name="..." />. Defaults to ID or FName.
	Type          string   // <input type="..." /> (or "select" or "textarea").
	Required      bool     // required=""
	Pattern       string   // pattern="..." (enforced on server as well)
	MaxLength     int      // 0: no max length
	Size          int      // field size. cols for textarea
	VSize         int      // rows for textarea
	FormatMessage string   // validation error format message
	Classes       []string // HTML classes
	ID            string   // HTML ID
	Autofocus     bool     // <input autofocus="" />
	Placeholder   string   // <input placeholder="..." />
	Label         string   // Label text
	ValueSet      string   // <select> value set.
}

// An option for a <select> field.
type SelectValue struct {
	// The actual value of the option sent to the server.
	Value string

	// The label shown to the user.
	Title string

	// Optgroup. Values in the same optgroup must be contiguous.  Optgroups
	// cannot be nested. Values with a Group of "" are outside any optgroup.
	Group string

	// Should the option be disabled?
	Disabled bool
}

// Obtains select values.
// The map is keyed by ValueSet. Return a list of values in the desired order.
var SelectValueFuncs = map[string]func(fi *FieldInfo, req *http.Request) []SelectValue{}

// Get the information for a struct field.
//
// The format is as follows:
//
//   Field  SomeType    `form:"TYPE[,required][,autofocus][,.htmlClass][,#htmlID][,<int>][,<int>!]"`
//
// The integer tag expresses a field size. The integer tag with "!" expresses a maximum length.
//
// TYPE is the HTML field type. For non-<input/> types, use the tag name ("select", "textarea", etc.)
//
// Many HTML classes may be specified.
// If the HTML ID is not specified, the struct field name is used as the field name and HTML ID.
// Otherwise, the HTML ID is used as the field name and HTML ID.
//
// The following additional field tags are also supported:
//
//   fmsg:          Format message, shown on validation error.
//   pattern:       Regexp to enforce on client and server.
//   placeholder:   Placeholder string.
//   label:         Label string.
//   set:           <select> value set.
//
func GetFieldInfo(sf reflect.StructField) FieldInfo {
	fi := FieldInfo{
		FName: sf.Name,
	}

	tags := strings.Split(sf.Tag.Get("form"), ",")
	fi.Type = tags[0]
	for _, v := range tags[1:] {
		switch {
		case v == "required":
			fi.Required = true
		case v == "autofocus":
			fi.Autofocus = true
		case v == "":
		case v[0] == '.':
			fi.Classes = append(fi.Classes, v[1:])
		case v[0] == '#':
			fi.ID = v[1:]
		case v[0] >= '0' && v[0] <= '9':
			strict := false
			if v[len(v)-1] == '!' {
				strict = true
				v = v[0 : len(v)-1]
			} else if idx := strings.IndexByte(v, 'x'); idx >= 0 {
				v2 := v[idx+1:]
				n, err := strconv.ParseUint(v2, 10, 31)
				if err == nil {
					fi.VSize = int(n)
				}

				v = v[0:idx]
			}

			n, err := strconv.ParseUint(v, 10, 31)
			if err == nil {
				if strict {
					fi.MaxLength = int(n)
				} else {
					fi.Size = int(n)
				}
			}
		default:
			if fi.Type == "" {
				fi.Type = v
			}
		}
	}

	fi.FormatMessage = sf.Tag.Get("fmsg")
	fi.Pattern = sf.Tag.Get("pattern")
	fi.ValueSet = sf.Tag.Get("set")

	if fi.ID == "" {
		fi.Name = fi.FName
		fi.Name = snaker.CamelToSnake(fi.Name)
		fi.ID = fi.Name
	} else {
		fi.Name = fi.ID
	}

	fi.Label = sf.Tag.Get("label")
	if fi.Label == "-" {
		fi.Label = ""
	} else if fi.Label == "" {
		fi.Label = fi.FName
	}

	fi.Placeholder = sf.Tag.Get("placeholder")
	if fi.Placeholder == "-" {
		fi.Placeholder = ""
	} else if fi.Placeholder == "" {
		fi.Placeholder = fi.Label
	}

	return fi
}
