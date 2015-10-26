package tpl

import "os"
import "path/filepath"
import "github.com/flosch/pongo2"
import "fmt"
import "github.com/hlandau/xlog"
import "net/http"
import "github.com/hlandau/degoutils/web/miscctx"

var log, Log = xlog.New("web.tpl")

var templates = map[string]*pongo2.Template{}

func GetTemplate(name string) *pongo2.Template {
	return templates[name]
}

func loadTemplates(dirname string) (map[string]*pongo2.Template, error) {
	dir, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return nil, err
	}

	compiled := make(map[string]*pongo2.Template)
	for _, f := range files {
		fn := f.Name()
		fext := filepath.Ext(fn)
		path := filepath.Join(dirname, fn)

		if f.IsDir() {
			subcompiled, err := loadTemplates(path)
			if err != nil {
				return nil, err
			}

			for k, v := range subcompiled {
				compiled[filepath.Join(fn, k)] = v
			}
		} else if fext == ".p2" {
			tpl, err := pongo2.FromFile(path)
			if err != nil {
				return nil, err
			}

			k := fn[0 : len(fn)-len(fext)]
			compiled[k] = tpl
		}
	}

	return compiled, nil
}

func LoadTemplates(dirname string) error {
	c, err := loadTemplates(dirname)
	if err != nil {
		return err
	}

	for k, v := range c {
		templates[k] = v
	}

	return nil
}

var ErrNotFound = fmt.Errorf("tpl not found")

func Show(req *http.Request, name string, args map[string]interface{}) {
	err := TryShow(req, name, args)
	log.Panice(err, "cannot render template")
}

var GetContextFunc func(req *http.Request) interface{}

func TryShow(req *http.Request, name string, args map[string]interface{}) error {
	tpl, ok := templates[name]
	if !ok {
		return ErrNotFound
	}

	if args == nil {
		args = map[string]interface{}{}
	}

	rw := miscctx.GetResponseWriter(req)
	args["c"] = GetContextFunc(req)

	err := tpl.ExecuteWriter(args, rw)
	if err != nil {
		return err
	}

	return nil
}
