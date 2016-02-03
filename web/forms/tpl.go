package forms

import "github.com/hlandau/degoutils/web/tpl"

func (fstate *State) MustShow(tplName string, args map[string]interface{}) {
	if args == nil {
		args = map[string]interface{}{}
	}

	args["f"] = fstate.f
	args["errors"] = &fstate.Errors
	tpl.MustShow(fstate.req, tplName, args)
}
