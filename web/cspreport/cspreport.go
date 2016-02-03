// Package cspreport provides facilities for logging CSP and HPKP violation
// reports.
package cspreport

import "time"
import "net/http"
import "encoding/json"
import "github.com/hlandau/xlog"

// Logger which generates CSP and HPKP reports.
var log, Log = xlog.New("web.cspreport")

// HTTP handler which logs CSP and HPKP reports.
var Handler http.Handler

func init() {
	Handler = http.HandlerFunc(handler)
}

func handler(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("Content-Type") != "application/csp-report" {
		pkpHandler(rw, req)
		return
	}

	r := CSPReport{}
	err := json.NewDecoder(req.Body).Decode(&r)
	if err != nil {
		rw.WriteHeader(400)
		return
	}

	log.Errorf("CSP Violation: %#v", &r)
	rw.WriteHeader(204)
}

func pkpHandler(rw http.ResponseWriter, req *http.Request) {
	r := PKPReport{}
	err := json.NewDecoder(req.Body).Decode(&r)
	if err != nil {
		rw.WriteHeader(400)
		return
	}

	if r.DateTime.IsZero() {
		rw.WriteHeader(415)
		return
	}

	log.Errorf("HPKP Violation: %#v", &r)
	rw.WriteHeader(204)
}

// CSP report structure.
type CSPReport struct {
	Body struct {
		BlockedURI         string `json:"blocked-uri"`
		DocumentURI        string `json:"document-uri"`
		EffectiveDirective string `json:"effective-directive"`
		OriginalPolicy     string `json:"original-policy"`
		Referrer           string `json:"referrer"`
		StatusCode         int    `json:"status-code"`
		ViolatedDirective  string `json:"violated-directive"`

		SourceFile   string `json:"source-file"`
		LineNumber   int    `json:"line-number"`
		ColumnNumber int    `json:"column-number"`
	} `json:"csp-report"`
}

// HPKP report structure.
type PKPReport struct {
	DateTime                  time.Time `json:"date-time"`
	Hostname                  string    `json:"hostname"`
	Port                      int       `json:"port"`
	EffectiveExpirationDate   time.Time `json:"effective-expiration-date"`
	IncludeSubdomains         bool      `json:"include-subdomains"`
	KnownPins                 []string  `json:"known-pins"`
	ServedCertificateChain    []string  `json:"served-certificate-chain"`
	ValidatedCertificateChain []string  `json:"validated-certificate-chain"`
}
