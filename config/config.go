package config

import "flag"
import "reflect"
import "strings"
import "strconv"
import "fmt"
import "unsafe"
import "github.com/BurntSushi/toml"
import "io/ioutil"
import "os"
import "path/filepath"

type Config struct {
	Bind       string
	PublicKey  string
	PrivateKey string
}

type Configurator struct {
	ProgramName        string
	invokedProgramName string

	ConfigFilePaths []string
	//Interspersed bool

	configFilePath string // path of the config file that was actually used, or ""

	rargs  []string
	target interface{}
	done   bool
}

// Returns the path to the config file which was actually used, or "" if no
// config file was used.
func (cc *Configurator) ConfigFilePath() string {
	return cc.configFilePath
}

/*func (cc *Configurator) parseArgs(f func(optName, optValue string, noValue, isShort bool) error) error {
  cc.rargs = []string{}
  args := os.Args[1:]

  // from pflags
  for len(args) > 0 {
    s := args[0]
    args = args[1:]

    if len(s) <= 1 || s[0] != '-' {
      // non-flag argument
      cc.rargs = append(cc.rargs, s)

      if !cc.Interspersed {
        cc.rargs = append(cc.rargs, args...)
        return nil
      }

      continue
    }

    if len(s) == 2 && s[1] == '-' {
      // --
      cc.rargs = append(f.rargs, args...)
      return nil
    }

    if s[1] == '-' {
      // long option, turn into short option
      s = s[1:]
    }


    name := s[1:]
    if name[0] == '-' || name[0] == '=' {
      return fmt.Errorf("bad flag syntax")
    }

    split := strings.SplitN(name, "=", 2)
    name = split[0]

    var err error
    if len(split) == 1 {
      err = f(name, "true", true, false)
    } else {
      err = f(name, split[1], false, false)
    }

    if err != nil {
      return err
    }
  }
  return nil
}*/

/*func (cc *Configurator) parse(target interface{}) error {
  cc.target = target
  cc.invokedProgramName = os.Args[0]

  err := cc.parseArgs(func(optName, optValue string, noValue, isShort bool) error {
    return cc.Set(optName, optValue)
  })

  if err != nil {
    return err
  }

  return nil
}*/

var configFile string
var exePath string

func init() {
	flag.StringVar(&configFile, "config", "", "path to configuration file")

	// We have to determine the absolute path of the executable before the CWD
	// (to which os.Args[0] is potentially relative) is changed, so we do it now.
	var err error
	exePath, err = filepath.Abs(os.Args[0])
	if err != nil {
		panic("cannot determine absolute path of executable filename")
	}
}

func resolvePath(p string) string {
	if !strings.HasPrefix(p, "$BIN/") {
		return p
	}

	return filepath.Join(filepath.Dir(exePath), p[5:])
}

func (cc *Configurator) ParseFatal(target interface{}) {
	if cc.done {
		return
	}

	cc.parseFatal(target, false)

	cc.done = true
}

func (cc *Configurator) buildPaths() {
	cc.ConfigFilePaths = []string{
		fmt.Sprintf("$BIN/../etc/%s/%s.conf", cc.ProgramName),
		fmt.Sprintf("$BIN/../etc/%s.conf", cc.ProgramName),
		fmt.Sprintf("/etc/%s/%s.conf", cc.ProgramName),
		fmt.Sprintf("/etc/%s.conf", cc.ProgramName),
	}
}

func (cc *Configurator) parseFatal(target interface{}, noVars bool) {
	if cc.ConfigFilePaths == nil {
		cc.buildPaths()
	}

	t := reflect.TypeOf(target)
	v := reflect.ValueOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = reflect.Indirect(v)
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("configurator target interface is not a struct: %s", t))
	}

	nf := t.NumField()
	for i := 0; i < nf; i++ {
		f := t.Field(i)
		name := strings.ToLower(f.Name)
		usage := f.Tag.Get("usage")
		dflt := f.Tag.Get("default")
		if usage == "" && dflt == "" {
			continue
		}
		vf := v.FieldByIndex(f.Index)
		switch f.Type.Kind() {
		case reflect.Int:
			dflti, err := strconv.ParseInt(dflt, 0, 32)
			if err != nil {
				panic("bad default value")
			}
			// set the default, and make sure this is writable at the same time
			vf.SetInt(int64(dflti))
			if !noVars {
				flag.IntVar((*int)(unsafe.Pointer(vf.UnsafeAddr())), name, int(dflti), usage)
			}
		case reflect.String:
			// set the default, and make sure this is writable at the same time
			vf.SetString(dflt)
			if !noVars {
				flag.StringVar((*string)(unsafe.Pointer(vf.UnsafeAddr())), name, dflt, usage)
			}
		case reflect.Bool:
			dfltb, err := strconv.ParseBool(dflt)
			if err != nil {
				panic("bad default value")
			}
			vf.SetBool(dfltb)
			if !noVars {
				flag.BoolVar((*bool)(unsafe.Pointer(vf.UnsafeAddr())), name, dfltb, usage)
			}
		default:
			panic("unsupported type")
		}
	}

	flag.Parse()

	cfiles := []string{}
	if configFile != "" {
		cfiles = append(cfiles, configFile)
	}
	for _, cf := range cc.ConfigFilePaths {
		cf = resolvePath(cf)
		cfiles = append(cfiles, cf)
	}

	if len(cfiles) > 0 {
		for _, cfn := range cfiles {
			fbuf, err := ioutil.ReadFile(cfn)
			if err != nil {
				if cfn == configFile {
					// print error if config file was specified on command line
					fmt.Printf("Error: cannot read config file \"%s\": %v", cfn, err)
				}
				continue
			}

			cc.configFilePath = cfn
			fdata := string(fbuf)

			_, err = toml.Decode(fdata, target)
			if err != nil {
				fmt.Printf("Cannot decode configuration file: %s", err)
				os.Exit(1)
			}

			// read only one configuration file
			break
		}
	}

	// Flags may have been overridden by the config file, so we have to parse the flags again.
	flag.Parse()
}
