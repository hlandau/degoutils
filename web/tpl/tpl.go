// Package tpl provides facilities for loading and displaying templates.
package tpl

import "path/filepath"
import "github.com/flosch/pongo2"
import "fmt"
import "github.com/hlandau/xlog"
import "net/http"
import "github.com/hlandau/degoutils/web/miscctx"
import "github.com/hlandau/degoutils/web/opts"
import "github.com/hlandau/degoutils/vfs"
import "github.com/hlandau/degoutils/binarc"
import "io"

var log, Log = xlog.New("web.tpl")

// Loaded templates.
var templates = map[string]*pongo2.Template{}

// Try to find a template with the given name. Returns nil if there is no such
// template loaded.
func GetTemplate(name string) *pongo2.Template {
	return templates[name]
}

// Load all templates from the given directory. The templates must have the file extension ".p2".
func LoadTemplates(dirname string) error {
	err := binarc.Setup(opts.BaseDir)
	if err != nil {
		return err
	}

	pongo2.OpenFunc = func(name string) (io.ReadCloser, error) {
		return vfs.Open(name)
	}

	c, err := loadTemplates(dirname)
	if err != nil {
		return err
	}

	for k, v := range c {
		templates[k] = v
	}

	return nil
}

func loadTemplates(dirname string) (map[string]*pongo2.Template, error) {
	dir, err := vfs.Open(dirname)
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

// Returned by TryShow if a template is not found.
var ErrNotFound = fmt.Errorf("template not found")

// Show the template with the given name and args. Failure to render the template results
// in a panic, which will ultimately result in a 500 response.
func MustShow(req *http.Request, name string, args map[string]interface{}) {
	err := Show(req, name, args)
	log.Panice(err, "cannot render template ", name)
}

// If this is non-nil, the value returned by this function will always be set
// as "c" in the args passed to Show.
var GetContextFunc func(req *http.Request) interface{}

// Try to show the template with the given name and args. Return an error on failure.
//
// The error might be ErrNotFound.
func Show(req *http.Request, name string, args map[string]interface{}) error {
	tpl, ok := templates[name]
	if !ok {
		return ErrNotFound
	}

	if args == nil {
		args = map[string]interface{}{}
	}

	if GetContextFunc != nil {
		args["c"] = GetContextFunc(req)
	}

	rw := miscctx.GetResponseWriter(req)
	err := tpl.ExecuteWriter(args, rw)
	if err != nil {
		return err
	}

	miscctx.SetCanOutputTime(req)
	return nil
}
