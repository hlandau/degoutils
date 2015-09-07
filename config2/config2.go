package config2

import "os"
import "fmt"
import "gopkg.in/hlandau/configurable.v0"
import "gopkg.in/hlandau/configurable.v0/cstruct"
import "gopkg.in/hlandau/configurable.v0/adaptflag"
import "gopkg.in/hlandau/configurable.v0/adaptconf"
import flag "github.com/ogier/pflag"

type Configurator struct {
	ProgramName    string
	configFilePath string
}

func (cfg *Configurator) ParseFatal(tgt interface{}) {
	c := cstruct.MustNew(tgt, cfg.ProgramName)
	configurable.Register(c)
	adaptflag.Adapt()
	flag.Parse()
	err := adaptconf.Load(cfg.ProgramName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load configuration file: %v\n", err)
		os.Exit(1)
	}

	cfg.configFilePath = adaptconf.LastConfPath()
}

func (cfg *Configurator) ConfigFilePath() string {
	return cfg.configFilePath
}
