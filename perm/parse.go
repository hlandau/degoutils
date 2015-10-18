package perm

import "regexp"
import "strings"
import "fmt"
import "strconv"
import "os"
import "io/ioutil"

var re_tuple = regexp.MustCompilePOSIX(`^([a-z0-9._:-]+)\(([0-9]+)\)`)

// Parse a permission string such as "some-permission(5)".
func ParsePermission(permission string) (Permission, error) {
	p, rest, err := parsePermission(permission)
	if err != nil {
		return Permission{}, err
	}

	if rest != "" {
		return Permission{}, fmt.Errorf("trailing data in permission string: %#v", rest)
	}

	return p, nil
}

// Parse a condition string such as "some-permission(5)".
func ParseCondition(condition string) (Condition, error) {
	p, rest, err := parseCondition(condition)
	if err != nil {
		return Condition{}, err
	}

	if rest != "" {
		return Condition{}, fmt.Errorf("trailing data in condition string: %#v", rest)
	}

	return p, nil
}

// Parses an implications string such as "a(1) => b(2), c(3) => d(4)".
//
// Implications may be separated by commas or newlines.
func ParseImplications(implications string) (ImplicationSet, error) {
	return parseImplications(implications)
}

func parseImplications(implications string) (is ImplicationSet, err error) {
	rest := implications
	for {
		rest = skipws(rest)
		if rest == "" {
			break
		}

		var impl Implication
		impl, rest, err = parseImplication(rest)
		if err != nil {
			return
		}

		is = append(is, impl)
		rest = skiphws(rest)
		if rest == "" {
			break
		}
		if rest[0] != ',' && rest[0] != '\n' {
			err = fmt.Errorf("expected comma or newline after implication, got %#v", rest)
			return
		}
		rest = rest[1:]
	}

	return
}

func parseImplication(implication string) (impl Implication, rest string, err error) {
	rest = implication
	rest = skiphws(rest)

	cond, rest, err := parseCondition(rest)
	if err != nil {
		return
	}

	rest = skiphws(rest)
	if len(rest) < 2 || rest[0] != '=' || rest[1] != '>' {
		err = fmt.Errorf("expected '=>' in implication string")
		return
	}

	rest = rest[2:]
	rest = skiphws(rest)

	perm, rest, err := parsePermission(rest)
	if err != nil {
		return
	}

	rest = skiphws(rest)
	impl = Implication{
		Condition:         cond,
		ImpliedPermission: perm,
	}
	return
}

func skiphws(s string) string {
	return strings.TrimLeft(s, " \t\r")
}

func skipws(s string) string {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if s != "" && s[0] == '#' {
			// skip comments
			idx := strings.IndexByte(s, '\n')
			if idx < 0 {
				return ""
			}

			s = s[idx+1:]
			continue
		}

		return s
	}
}

func parsePermission(permission string) (p Permission, rest string, err error) {
	name, level, rest, err := parseTuple(permission)
	if err != nil {
		return Permission{}, "", err
	}

	return Permission{
		Name:  name,
		Level: level,
	}, rest, nil
}

func parseCondition(condition string) (p Condition, rest string, err error) {
	name, level, rest, err := parseTuple(condition)
	if err != nil {
		return Condition{}, "", err
	}

	return Condition{
		Name:     name,
		MinLevel: level,
	}, rest, nil
}

// Parse a (string, int)-tuple string such as "some-permission(5)".
func parseTuple(tuple string) (name string, level int, rest string, err error) {
	m := re_tuple.FindStringSubmatch(tuple)
	if m == nil {
		err = fmt.Errorf("invalid permission/condition string: %#v", tuple)
		return
	}

	name = m[1]
	levels := m[2]
	leveln, err := strconv.ParseInt(levels, 10, 32)
	if err != nil {
		return
	}

	level = int(leveln)
	rest = tuple[len(m[0]):]
	return
}

// Load implications in the textual form from a file.
func LoadImplicationsFromFile(filename string) (ImplicationSet, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return ParseImplications(string(data))
}
