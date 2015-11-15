package web

import "net"
import "net/http"
import "net/url"
import "strings"
import "github.com/hlandau/degoutils/web/opts"
import "github.com/hlandau/degoutils/web/origin"
import "github.com/hlandau/degoutils/web/session"
import "github.com/hlandau/degoutils/web/errorhandler"
import "github.com/hlandau/degoutils/web/servicenexus"
import "github.com/hlandau/degoutils/web/tpl"
import "github.com/hlandau/degoutils/web/miscctx"
import "github.com/gorilla/context"
import "gopkg.in/hlandau/easymetric.v1/cexp"
import "gopkg.in/tylerb/graceful.v1"
import "time"
import "fmt"
import "path/filepath"
import "github.com/hlandau/captcha"
import "github.com/hlandau/xlog"
import "github.com/hlandau/degoutils/health"
import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "github.com/garyburd/redigo/redis"
import "github.com/hlandau/degoutils/web/session/storage"
import "github.com/hlandau/degoutils/web/session/storage/memorysession"
import "github.com/hlandau/degoutils/web/session/storage/redissession"
import "github.com/hlandau/degoutils/web/assetmgr"
import "github.com/hlandau/degoutils/web/cspreport"
import "github.com/llgcode/draw2d"

var log, Log = xlog.New("web")

var cRequestsHandled = cexp.NewCounter("web.requestsHandled")

var bindFlag = cflag.String(nil, "bind", ":3400", "HTTP binding address")
var redisAddressFlag = cflag.String(nil, "redisaddress", "localhost:6379", "Redis address")
var redisPasswordFlag = cflag.String(nil, "redispassword", "", "Redis password")
var redisPrefixFlag = cflag.String(nil, "redisprefix", "", "Redis prefix")
var captchaFontPathFlag = cflag.String(nil, "captchafontpath", "", "Path to CAPTCHA font directory")
var reportURI = cflag.String(nil, "reporturi", "/.csp-report", "CSP/PKP report URI")

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
	AssetMgr      *assetmgr.Manager
}

func (cfg *Config) GetCAPTCHA() *captcha.Config {
	return cfg.CAPTCHA
}

var ServerKey int

func (cfg *Config) Handler(h http.Handler) http.Handler {
	cfg.mustInit()

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
	return context.ClearHandler(errorhandler.Handler(mux))
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

	RedirectTo(req, 308, newURL.String())
}

func (cfg *Config) Listen(h http.Handler) error {
	cfg.mustInit()
	cfg.HTTPServer.Handler = cfg.Handler(h)
	if opts.DevMode {
		cfg.HTTPServer.Timeout = 10 * time.Millisecond
	} else {
		cfg.HTTPServer.Timeout = 30 * time.Second
	}
	cfg.HTTPServer.NoSignalHandling = true

	var err error
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

func (cfg *Config) Init() error {
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

	cfg.AssetMgr, err = assetmgr.New(assetmgr.Config{
		Path: filepath.Join(opts.BaseDir, bstaticName),
	})
	if err != nil {
		return err
	}

	cfg.inited = true
	return nil
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
