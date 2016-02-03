package web

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/hlandau/captcha"
	"github.com/hlandau/degoutils/health"
	"github.com/hlandau/degoutils/web/assetmgr"
	"github.com/hlandau/degoutils/web/cspreport"
	"github.com/hlandau/degoutils/web/errorhandler"
	"github.com/hlandau/degoutils/web/miscctx"
	"github.com/hlandau/degoutils/web/opts"
	"github.com/hlandau/degoutils/web/origin"
	"github.com/hlandau/degoutils/web/servicenexus"
	"github.com/hlandau/degoutils/web/session"
	"github.com/hlandau/degoutils/web/session/storage"
	"github.com/hlandau/degoutils/web/session/storage/memorysession"
	"github.com/hlandau/degoutils/web/session/storage/redissession"
	"github.com/hlandau/degoutils/web/tpl"
	"github.com/hlandau/degoutils/web/weberror"
	"github.com/hlandau/xlog"
	"github.com/llgcode/draw2d"
	"gopkg.in/hlandau/easyconfig.v1/cflag"
	"gopkg.in/hlandau/easymetric.v1/cexp"
	"gopkg.in/tylerb/graceful.v1"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

var log, Log = xlog.New("web")

var cRequestsHandled = cexp.NewCounter("web.requestsHandled")

var bindFlag = cflag.String(nil, "bind", ":3400", "HTTP binding address")
var redisAddressFlag = cflag.String(nil, "redisaddress", "localhost:6379", "Redis address")
var redisPasswordFlag = cflag.String(nil, "redispassword", "", "Redis password")
var redisPrefixFlag = cflag.String(nil, "redisprefix", "", "Redis prefix")
var captchaFontPathFlag = cflag.String(nil, "captchafontpath", "", "Path to CAPTCHA font directory")
var reportURI = cflag.String(nil, "reporturi", "/.csp-report", "CSP/PKP report URI")

var Router *mux.Router

func init() {
	Router = mux.NewRouter()
	Router.KeepContext = true
	Router.NotFoundHandler = http.HandlerFunc(NotFound)
}

func NotFound(rw http.ResponseWriter, req *http.Request) {
	// handle static file serving here so we can let dynamic pages take priority
	// due to the "/:page" route and files like favicon.ico, there is an ambiguity
	// which means we need to let dynamic handlers go first. we use our own code
	// instead of gocraft/web's static file middleware since we want to do Expires
	// headers, etc.
	err := assetmgr.Default.TryHandle(rw, req)
	if err != nil {
		weberror.Show(req, 404)
	}
}

type Config struct {
	SessionConfig *session.Config
	Server        interface{}
	NoForceSSL    bool
	StripWWW      bool
	BaseURL       string
	HTTPServer    graceful.Server
	httpListener  net.Listener
	CAPTCHA       *captcha.Config
	stopping      bool
	statusChan    chan string
	criterion     *health.Criterion
	rpool         redis.Pool
	inited        bool
}

func (cfg *Config) GetCAPTCHA() *captcha.Config {
	return cfg.CAPTCHA
}

var ServerKey int

func (cfg *Config) Handler(h http.Handler) http.Handler {
	cfg.mustInit()

	// TODO: nonce?
	csp := "default-src 'self' https://www.google-analytics.com; frame-ancestors 'none'; img-src 'self' https://www.google-analytics.com data:; form-action 'self'; plugin-types;"
	if reportURI.Value() != "" {
		csp += fmt.Sprintf(" report-uri %s;", reportURI.Value())
	}

	var h2 http.Handler = http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		cRequestsHandled.Inc()

		miscctx.SetResponseWriter(rw, req)
		context.Set(req, &ServerKey, cfg.Server)

		hdr := rw.Header()
		hdr.Set("X-Frame-Options", "DENY")
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-UA-Compatible", "ie=edge")
		hdr.Set("Content-Security-Policy", csp)
		if origin.IsSSL(req) {
			hdr.Set("Strict-Transport-Security", "max-age=15552000")
		}

		if !opts.DevMode && !cfg.NoForceSSL && !origin.IsSSL(req) {
			cfg.redirectHTTPS(rw, req)
			return
		}

		if cfg.StripWWW && strings.HasPrefix(req.Host, "www.") {
			cfg.redirectStripWWW(rw, req)
			return
		}

		h.ServeHTTP(rw, req)
	})

	if cfg.SessionConfig != nil {
		h2 = cfg.SessionConfig.InitHandler(h2)
	}

	if cfg.CAPTCHA == nil {
		cfg.CAPTCHA = &captcha.Config{
			DisallowHandlerNew: true,
			Leeway:             1,
		}

		if captchaFontPathFlag.Value() != "" {
			cfg.CAPTCHA.SetFontPath(captchaFontPathFlag.Value())
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", h2)
	mux.Handle("/.captcha/", cfg.CAPTCHA.Handler("/.captcha/"))
	mux.Handle("/.csp-report", cspreport.Handler)
	mux.Handle("/.service-nexus/", servicenexus.Handler(h2))
	return context.ClearHandler(timingHandler(errorhandler.Handler(methodOverride(mux))))
}

func isValidOverrideMethod(methodName string) bool {
	switch methodName {
	case "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func determineOverrideMethod(req *http.Request) string {
	if req.Method != "POST" {
		return req.Method
	}

	m := req.Header.Get("X-HTTP-Method-Override")
	if m == "" {
		// XXX: this reads the POST data, which might be a problem
		m = req.PostFormValue("_method")
	}

	if isValidOverrideMethod(m) {
		return m
	}

	return "POST"
}

func methodOverride(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req.Method = determineOverrideMethod(req)
		h.ServeHTTP(rw, req)
	})
}

func timingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		h.ServeHTTP(rw, req)
		totalTimeTaken := time.Since(startTime)

		if miscctx.GetCanOutputTime(req) {
			fmt.Fprintf(rw, "<!-- %v -->", totalTimeTaken)
		}
	})
}

func (cfg *Config) redirectHTTPS(rw http.ResponseWriter, req *http.Request) {
	cfg.redirectCanonicalize(req, func(u *url.URL) {
		u.Scheme = "https"
	})
}

func (cfg *Config) redirectStripWWW(rw http.ResponseWriter, req *http.Request) {
	cfg.redirectCanonicalize(req, func(u *url.URL) {
		u.Host = strings.TrimPrefix(u.Host, "www.")
	})
}

func (cfg *Config) redirectCanonicalize(req *http.Request, transformFunc func(u *url.URL)) {
	newURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		newURL = &url.URL{}
		newURL.Host = req.Host
	}
	newURL.Path = req.URL.Path
	newURL.RawQuery = req.URL.RawQuery
	transformFunc(newURL)

	miscctx.RedirectTo(req, 308, newURL.String())
}

func (cfg *Config) Listen() error {
	err := cfg.init()
	if err != nil {
		return err
	}

	cfg.HTTPServer.Handler = cfg.Handler(Router)
	if opts.DevMode {
		cfg.HTTPServer.Timeout = 10 * time.Millisecond
	} else {
		cfg.HTTPServer.Timeout = 30 * time.Second
	}
	cfg.HTTPServer.NoSignalHandling = true

	cfg.httpListener, err = net.Listen("tcp", cfg.HTTPServer.Addr)
	if err != nil {
		return err
	}

	return nil
}

func (cfg *Config) Serve() error {
	if cfg.httpListener == nil {
		return fmt.Errorf("must call Listen first")
	}

	return cfg.HTTPServer.Serve(cfg.httpListener)
}

func (cfg *Config) init() error {
	if cfg.inited {
		return nil
	}

	var err error
	if redisAddressFlag.Value() != "" {
		cfg.rpool.Dial = func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisAddressFlag.Value())
			if err != nil {
				return nil, err
			}

			if redisPasswordFlag.Value() != "" {
				if _, err := c.Do("AUTH", redisPasswordFlag.Value()); err != nil {
					c.Close()
					return nil, err
				}
			}

			return c, nil
		}
		cfg.rpool.MaxIdle = 2
	}

	cfg.statusChan = make(chan string, 8)
	cfg.criterion = health.NewCriterion("web.ok", false)
	if cfg.HTTPServer.Server == nil {
		cfg.HTTPServer.Server = &http.Server{}
	}
	if cfg.HTTPServer.Addr == "" {
		cfg.HTTPServer.Addr = bindFlag.Value()
	}
	if cfg.SessionConfig == nil {
		cfg.SessionConfig = &session.Config{}
	}
	if cfg.SessionConfig.CookieName == "" {
		cfg.SessionConfig.CookieName = "s"
	}
	if len(cfg.SessionConfig.SecretKey) == 0 {
		cfg.SessionConfig.SecretKey = opts.VariantSecretKey("cookie-secret-key")
	}
	if cfg.SessionConfig.Store == nil {
		var redisStore storage.Store
		if redisAddressFlag.Value() != "" {
			if redisPrefixFlag.Value() == "" {
				return fmt.Errorf("must specify a redis prefix")
			}
			redisStore, err = redissession.New(redissession.Config{
				Prefix: redisPrefixFlag.Value() + "s/",
				GetConn: func() (redis.Conn, error) {
					c := cfg.rpool.Get()
					if c == nil {
						return nil, fmt.Errorf("cannot get redis")
					}

					return c, nil
				},
			})
			if err != nil {
				return err
			}
		}

		cfg.SessionConfig.Store, err = memorysession.New(memorysession.Config{
			FallbackStore: redisStore,
		})
		if err != nil {
			return err
		}
	}

	draw2d.SetFontFolder(filepath.Join(opts.BaseDir, "assets/fonts"))

	err = tpl.LoadTemplates(filepath.Join(opts.BaseDir, "tpl"))
	if err != nil {
		return err
	}

	bstaticName := "bstatic"
	if !opts.DevMode {
		bstaticName = "bstatic.rel"
	}

	assetmgr.Default, err = assetmgr.New(assetmgr.Config{
		Path: filepath.Join(opts.BaseDir, bstaticName),
	})
	if err != nil {
		return err
	}

	Router.HandleFunc("/{page}", Front_GET).Methods("GET")
	Router.HandleFunc("/", Front_GET).Methods("GET")

	cfg.inited = true
	return nil
}

func Front_GET(rw http.ResponseWriter, req *http.Request) {
	page := mux.Vars(req)["page"]
	var err error
	if page == "" {
		err = tpl.Show(req, "front/index", nil)
	} else {
		err = tpl.Show(req, "front/"+page, nil)
	}

	if err == tpl.ErrNotFound {
		NotFound(rw, req)
	}
}

func (cfg *Config) mustInit() {
	if !cfg.inited {
		log.Fatal("must call Init()")
	}
}

func (cfg *Config) getStatusChan() chan string {
	cfg.mustInit()
	return cfg.statusChan
}

func (cfg *Config) StatusChan() <-chan string {
	return cfg.getStatusChan()
}

func (cfg *Config) SetStatus(status string) {
	log.Debug("status: ", status)
	select {
	case cfg.getStatusChan() <- status:
	default:
	}
}

func (cfg *Config) Start() error {
	go func() {
		err := cfg.Serve()
		if !cfg.stopping {
			log.Fatale(err, "cannot listen")
		}
	}()

	cfg.mustInit()
	cfg.criterion.Inc()
	log.Debug("ready")
	return nil
}

func (cfg *Config) Stop() error {
	cfg.SetStatus("shutting down")
	close(cfg.getStatusChan())
	cfg.criterion.Dec()
	cfg.stopping = true
	cfg.HTTPServer.Stop(cfg.HTTPServer.Timeout)
	<-cfg.HTTPServer.StopChan()
	log.Debug("graceful shutdown complete")
	return nil
}
