package buildinfo

import "github.com/hlandau/buildinfo"

// Full build info.
var BuildInfo string

// Set via go build.
var RawBuildInfo string

// Program-settable extra version information to print.
var Extra string

func init() {
	if RawBuildInfo != "" {
		buildinfo.RawBuildInfo = RawBuildInfo
	}
	if BuildInfo != "" {
		buildinfo.BuildInfo = BuildInfo
	}
	if Extra != "" {
		buildinfo.Extra = Extra
	}
	buildinfo.Update()
	RawBuildInfo = buildinfo.RawBuildInfo
	BuildInfo = buildinfo.BuildInfo
	Extra = buildinfo.Extra
}

func Full() string {
	return buildinfo.Full()
}
