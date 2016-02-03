// Package opts provides miscellaneous configurable globals commonly required
// by web application servers.
package opts

import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "github.com/hlandau/xlog"
import "crypto/rand"
import "crypto/hmac"
import "crypto/sha256"
import "encoding/hex"

var log, Log = xlog.New("web.opts")

// Development mode? In development mode, the application server may behave
// differently. Errors may be reported differently, assets may be reloaded
// automatically, etc.
//
// Configurable 'devmode'.
var DevMode bool

var devModeFlag = cflag.BoolVar(nil, &DevMode, "devmode", false, "Development mode?")

// Base directory. Some files may be looked for relative to this directory; templates, assets, etc.
//
// Configurable 'basedir'.
var BaseDir string

var baseDirFlag = cflag.StringVar(nil, &BaseDir, "basedir", "", "Base dir (containing assets, tpl directories, etc.)")

// Base URL. This is the canonical base URL, which will be used in e.g. e.
// mails. It should not have a trailing slash.
//
// Configurable 'baseurl'.
var BaseURL string

var baseURLFlag = cflag.StringVar(nil, &BaseURL, "baseurl", "", "Base URL (e.g. https://DOMAIN)")

// Base domain. This may be used in circumstances in which an URL is not appropriate, but rather
// a domain name is needed.
//
// Configurable 'basedomain'.
var BaseDomain string

var baseDomainFlag = cflag.StringVar(nil, &BaseDomain, "basedomain", "", "Base domain name")

// Returns the application secret key. If none is configured, a random,
// temporary secret key is generated.
func SecretKey() []byte {
	if secretKey != nil {
		return secretKey
	}

	var err error
	s := secretKeyFlag.Value()
	if s == "" {
		secretKey = make([]byte, 32)
		_, err = rand.Read(secretKey)
		log.Panice(err)
	} else if len(s) == 64 {
		secretKey, err = hex.DecodeString(s)
		log.Fatale(err, "Cannot decode secret key string (must be hex)")
	} else {
		log.Fatale(err, "Secret key must be 64 hex characters")
	}

	return secretKey
}

// Gets a 32-byte variant secret key, derived using the given name.
func VariantSecretKey(name string) []byte {
	h := hmac.New(sha256.New, SecretKey())
	h.Write([]byte(name))
	return h.Sum(nil)
}

var secretKey []byte

var secretKeyFlag = cflag.String(nil, "secretkey", "", "Secret key (64 hex characters)")
