package xlogconfig

import "github.com/hlandau/dexlogconfig"

// Parse registered configurables and setup logging.
//
// Deprecated; use github.com/hlandau/dexlogconfig.Init instead. This just
// forwards the call for backwards compatibility.
func Init() {
	dexlogconfig.Init()
}
