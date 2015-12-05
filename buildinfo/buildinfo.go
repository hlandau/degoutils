package buildinfo

import "encoding/base64"
import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "os"
import "fmt"
import "runtime"

// Full build info.
var BuildInfo string

// Set via go build.
var RawBuildInfo string

// Program-settable extra version information to print.
var Extra string

func init() {
	versionFlag := cflag.Bool(nil, "version", false, "Print version information")
	versionFlag.RegisterOnChange(func(bf *cflag.BoolFlag) {
		if !bf.Value() {
			return
		}

		fmt.Fprint(os.Stderr, Full())
		os.Exit(2)
	})

	if RawBuildInfo == "" {
		return
	}

	b, err := base64.StdEncoding.DecodeString(RawBuildInfo)
	if err != nil {
		return
	}

	BuildInfo = string(b)
}

func Full() string {
	bi := BuildInfo
	if bi == "" {
		bi = "build unknown"
	}
	return fmt.Sprintf("%sgo version %s %s/%s %s cgo=%v\n%s\n", Extra, runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.Compiler, Cgo, bi)
}