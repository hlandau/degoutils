package opts

import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "github.com/hlandau/xlog"
import "crypto/rand"
import "crypto/hmac"
import "crypto/sha256"
import "encoding/hex"

var log, Log = xlog.New("web.opts")

// Development mode?
var DevMode bool

var devModeFlag = cflag.BoolVar(nil, &DevMode, "devmode", false, "Development mode?")

// Base directory?
var BaseDir string

var baseDirFlag = cflag.StringVar(nil, &BaseDir, "basedir", "", "Base dir (containing assets, tpl directories, etc.)")

// Base URL
var BaseURL string

var baseURLFlag = cflag.StringVar(nil, &BaseURL, "baseurl", "", "Base URL (e.g. https://DOMAIN)")

// Base Domain
var BaseDomain string

var baseDomainFlag = cflag.StringVar(nil, &BaseDomain, "basedomain", "", "Base domain name")

// Secret key?
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

// Gets a 32-byte variant secret key.
func VariantSecretKey(name string) []byte {
	h := hmac.New(sha256.New, SecretKey())
	h.Write([]byte(name))
	return h.Sum(nil)
}

var secretKey []byte

var secretKeyFlag = cflag.String(nil, "secretkey", "", "Secret key (64 hex characters)")
