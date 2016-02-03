package web

import (
	"gopkg.in/hlandau/easymetric.v1/adaptexpvar"
	"gopkg.in/hlandau/easymetric.v1/adaptprometheus"
)

func init() {
	adaptexpvar.Register()
	adaptprometheus.Register()
}
