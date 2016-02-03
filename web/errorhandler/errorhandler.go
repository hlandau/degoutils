// Package errorhandler provides a panic handler for HTTP requests which serves
// an error notice and optionally sends e. mail.
//
// Configurables:
//
//   'panicsto'          E. mail address to send panics to.
//                       "" (default): don't send e. mails.
//
//   'webmasteraddress'  Webmaster e. mail address, shown in error notices.
//                       "" (default): don't show webmaster e. mail address.
//
// Measurables:
//
//   'web.errorResponsesIssued'     Counter. Counts number of panics that the error handler
//                                  has caught.
//
package errorhandler

import "net/http"
import "runtime"
import "gopkg.in/yaml.v2"
import texttemplate "text/template"
import "html/template"
import "bytes"
import "time"
import "github.com/hlandau/degoutils/log"
import "github.com/hlandau/degoutils/sendemail"
import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "gopkg.in/hlandau/svcutils.v1/exepath"
import "fmt"
import "github.com/hlandau/degoutils/web/opts"
import "github.com/hlandau/degoutils/web/servicenexus"
import "net/url"
import "strings"
import "gopkg.in/hlandau/easymetric.v1/cexp"

var cErrorResponsesIssued = cexp.NewCounter("web.errorResponsesIssued")

var panicsToFlag = cflag.String(nil, "panicsto", "", "E. mail address to send panics to")
var webmasterAddressFlag = cflag.String(nil, "webmasteraddress", "", "Webmaster e. mail address (shown on panic)")

// The error handling HTTP handler, wrapping an inner handler for which panics
// are handled.
func Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				const size = 4096
				stack := make([]byte, size)
				stack = stack[:runtime.Stack(stack, false)]

				renderError(rw, req, err, stack)
			}
		}()

		h.ServeHTTP(rw, req)
	})
}

func errorMode(req *http.Request) (shouldEncrypt, shouldEmail bool) {
	shouldEncrypt = !opts.DevMode || !servicenexus.CanAccess(req)
	shouldEmail = panicsToFlag.Value() != "" && shouldEncrypt
	return
}

func renderError(rw http.ResponseWriter, req *http.Request, reqerr interface{}, stack []byte) {
	_, filePath, line, _ := runtime.Caller(4)

	cErrorResponsesIssued.Inc()

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Del("Content-Security-Policy")
	rw.Header().Del("Content-Security-Policy-Report-Only")
	rw.WriteHeader(http.StatusInternalServerError)

	type info struct {
		Error         interface{} `yaml:"error"`
		Stack         string      `yaml:"stack"`
		URL           string      `yaml:"url"`
		Method        string      `yaml:"method"`
		Proto         string      `yaml:"proto"`
		Header        http.Header `yaml:"header"`
		ContentLength int64       `yaml:"content-length"`
		RawRemoteAddr string      `yaml:"raw-remote-addr"`
		Host          string      `yaml:"host"`
		FilePath      string      `yaml:"file-path"`
		FileLine      int         `yaml:"file-line"`
		Time          string      `yaml:"time"`
		PostForm      url.Values  `yaml:"post-form"`
	}

	errInfo := info{
		Error:         reqerr,
		Stack:         strings.Replace(string(stack), "\t", "  ", -1),
		URL:           req.URL.String(),
		Method:        req.Method,
		Proto:         req.Proto,
		Header:        req.Header,
		ContentLength: req.ContentLength,
		RawRemoteAddr: req.RemoteAddr,
		Host:          req.Host,
		FilePath:      filePath,
		FileLine:      line,
		Time:          time.Now().String(),
	}
	if req.ContentLength >= 0 && req.ContentLength <= 4*1024 { // 4k size limit
		errInfo.PostForm = req.PostForm
	}

	b, err := yaml.Marshal(&errInfo)
	log.Infoe(err, "marshalling emergency error information") // ...

	shouldEncrypt, shouldEmail := errorMode(req)

	var encryptedError string
	if shouldEncrypt {
		encryptedError = encryptErrorBase64(b)
	}

	data := map[string]interface{}{
		"EncryptedBlob":    encryptedError,
		"Info":             string(b),
		"Encrypted":        shouldEncrypt,
		"WebmasterAddress": webmasterAddressFlag.Value(),
		"Notified":         shouldEmail,
	}

	emergencyErrorTemplate.Execute(rw, data)

	// send e. mail
	if shouldEmail {
		emailBuf := new(bytes.Buffer)
		emergencyErrorEmailTemplate.Execute(emailBuf, data)

		subjectLine := fmt.Sprintf("%s panic", exepath.ProgramName)

		sendemail.SendAsync(&sendemail.Email{
			To: []string{panicsToFlag.Value()},
			Headers: map[string][]string{
				"Subject": []string{subjectLine},
			},
			Body: string(emailBuf.Bytes()),
		})
	}
}

const emergencyErrorTpl = `<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Internal Server Error</title>
    <style type="text/css">
      body { font-family: "Arial", sans-serif; margin-right: 2em; }
      .oerror { float: left; margin: 2em; margin-right: 0; margin-bottom: 0; }
      .error { padding: 1em; padding-top: 0; padding-bottom: 0.1em;
        box-shadow: 10px 10px 0 #ff0000; border: solid 1px #ff0000;
        margin-bottom: 2em; }
      h1:first-child { margin-top: 0; }
      h1 > span { background-color: #ff0000; color: #fff; padding: 0 0.2em; }
      a:link, a:visited { color: #ff0000; }
      {{if .Encrypted}}.info { color: #aaa; }{{end}}
      .info > h2 { font-size: 0.8em; margin-bottom: 0; text-transform: lowercase; }
      pre { margin-top: 0.3em; }
    </style>
  </head>
  <body>
    <div class="oerror">
      <div class="error">
        <h1><span>Internal Server Error</span></h1>
        <p>An internal server error occurred while processing your request.</p>
        <p>{{if .Notified}}The webmaster has been notified. {{end}}Please try again later.</p>
        {{if .WebmasterAddress}}
        <p>For support, e. mail <a href="mailto:{{.WebmasterAddress}}">{{.WebmasterAddress}}</a>.</p>
        {{end}}
        <p><a href="/">Return to the homepage</a></p>
      </div>

      <div class="info">
        <h2>Error Information</h2>
        <pre>{{if .Encrypted}}{{.EncryptedBlob}}{{else}}{{.Info}}{{end}}</pre>
      </div>
    </div>
  </body>
</html>`

const emergencyErrorEmailTpl = `A panic has occurred.

Error information:
---------------------------------------------------------------------
{{.Info}}
---------------------------------------------------------------------
`

var emergencyErrorTemplate = template.Must(template.New("ErrorPage").Parse(emergencyErrorTpl))
var emergencyErrorEmailTemplate = texttemplate.Must(texttemplate.New("ErrorEmail").Parse(emergencyErrorEmailTpl))
