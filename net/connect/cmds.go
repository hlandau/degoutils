package connect

import "strings"
import "strconv"
import "errors"
import "fmt"

//import "github.com/hlandau/degoutils/log"
import "io"

const (
	cmdsMT_CONN = 0
	cmdsMT_FAIL = 1
)

type cmdsMethod struct {
	methodType         int
	name               string
	port               int
	implicitMethodName string
	explicitMethodName string
}

type cmdsApp struct {
	metaMethod string
	methods    []cmdsMethod
}

type cmdsInfo map[string]cmdsApp

func eBadCmds(s string) error {
	return errors.New(fmt.Sprintf("badly formatted cmds at \"%s\"", s))
}

func parseAppName(s string) (v, rest string, err error) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' || c == '-' || c == '+' || '0' <= c && c <= '9':
			if c < 'A' && c > 'Z' && c < 'a' && c > 'z' && i == 0 {
				err = eBadCmds(s)
				return
			}

		case c == '=':
			if i == 0 {
				err = eBadCmds(s)
				return
			}

			v = s[0:i]
			rest = s[i+1:]
			return

		default:
			err = eBadCmds(s[i:])
			return
		}
	}
	err = eBadCmds(s)
	return
}

func readXAlphaUntil(s string, until string) (v, rest string, err error) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case ('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' || c == '-' ||
			('0' <= c && c <= '9' && i != 0)):

		default:
			if strings.IndexByte(until, c) >= 0 {
				v = s[0:i]
				rest = s[i:] // include terminating sigil in rest
				return
			} else {
				err = eBadCmds(s)
				return
			}
		}
	}

	err = io.EOF
	v = s
	return
}

func readIntUntil(s string, until string) (v int, rest string, err error) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case '0' <= c && c <= '9':
			if c == '0' && i == 0 {
				err = eBadCmds(s)
				return
			}

		default:
			if strings.IndexByte(until, c) >= 0 {
				vs := s[0:i]
				var v64 int64
				if v64, err = strconv.ParseInt(vs, 10, 16); err != nil {
					return
				}
				v = int(v64)
				rest = s[i:] // include terminating sigil in rest
				return
			} else {
				err = eBadCmds(s)
				return
			}
		}
	}

	err = eBadCmds(s)
	return
}

func parseMethodInner(s string) (method cmdsMethod, rest string, err error) {
	if s == "" {
		err = eBadCmds(s)
		return
	}

	var mport int = -1
	var mname string

	if s[0] >= '0' && s[0] <= '9' {
		mport, rest, err = readIntUntil(s, "$+; ")
	} else {
		mname, rest, err = readXAlphaUntil(s, "$+; ")
	}

	if err == io.EOF {
		err = nil
	}

	if err != nil {
		return
	}

	if mname == "fail" {
		method.methodType = cmdsMT_FAIL
		if rest != "" && rest[0] != ';' && rest[0] != ' ' {
			err = eBadCmds(s)
		} else {
			err = nil
		}
		return
	}

	method.port = mport
	method.name = mname

	if rest == "" {
		err = eBadCmds(rest)
		return
	}

	for {
		if rest == "" {
			return
		}

		var mn string
		switch rest[0] {
		case '$':
			if method.implicitMethodName != "" {
				err = eBadCmds(rest)
				return
			}

			if mn, rest, err = readXAlphaUntil(rest[1:], " +$;"); err != nil && err != io.EOF {
				return
			}

			method.implicitMethodName = mn

		case '+':
			if method.explicitMethodName != "" {
				err = eBadCmds(rest)
				return
			}

			if mn, rest, err = readXAlphaUntil(rest[1:], " +$;"); err != nil && err != io.EOF {
				return
			}

			method.explicitMethodName = mn

		case ';':
			rest = rest[1:]
			return
		case ' ':
			return

		default:
			err = eBadCmds(s)
			return
		}
	}
}

func parseMethod(s string) (method cmdsMethod, rest string, err error) {
	method, rest, err = parseMethodInner(s)
	if err != nil && err != io.EOF {
		return
	}

	if method.methodType == cmdsMT_CONN {
		switch method.implicitMethodName {
		case "":
		case "tls":
		case "zmq":

		default:
			err = errors.New(fmt.Sprintf("Unsupported implicit method name in method descriptor: %+v", method))
			return
		}

		switch method.explicitMethodName {
		case "tcp":
		case "udp":

		default:
			err = errors.New(fmt.Sprintf("Unsupported explicit method name in method descriptor: %+v", method))
			return
		}
	}

	return
}

func parseApp(s string) (appName string, app cmdsApp, rest string, err error) {
	if appName, rest, err = parseAppName(s); err != nil {
		return
	}

	var metaName string
	if rest != "" && rest[0] == '@' {
		if metaName, rest, err = readXAlphaUntil(rest[1:], ";"); err != nil {
			return
		}
		rest = rest[1:]
		app.metaMethod = metaName
	}

	var method cmdsMethod
	for {
		if method, rest, err = parseMethod(rest); err != nil && err != io.EOF {
			return
		}

		app.methods = append(app.methods, method)
		if rest == "" || rest[0] == ' ' {
			rest = strings.TrimLeft(rest, " ")
			return
		}
	}
}

func parseCmds(s string) (info cmdsInfo, err error) {
	var appName string
	var app cmdsApp
	info = make(map[string]cmdsApp)

	for s != "" {
		if appName, app, s, err = parseApp(s); err != nil && err != io.EOF {
			break
		}
		//log.Info(fmt.Sprintf("CMDS: %s: %+v", appName, app))
		info[appName] = app
	}

	if err == io.EOF {
		err = nil
	}

	return
}
