package main

import _ "github.com/hlandau/degoutils/configurable/exampleapp/examplelib"
import "github.com/hlandau/degoutils/configurable/adaptflag"

//import "flag"
import flag "github.com/ogier/pflag"

func main() {
	adaptflag.AdaptWithFunc(func(v adaptflag.Value, name, usage string) {
		flag.Var(v, name, usage)
	})

	flag.Parse()
}
