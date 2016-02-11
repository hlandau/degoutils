// Package redissession provides a Redis-based session store.
package redissession

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/hlandau/degoutils/web/session/storage"
	"github.com/hlandau/xlog"
	"github.com/satori/go.uuid"
	"time"
)

var log, Log = xlog.New("web.session.redissession")

type sess struct {
	Data     map[string]interface{}
	LastSeen time.Time
}

// Redis-backed session store configuration.
type Config struct {
	// After what period of inactivity should sessions expire?
	//
	// Default: 4 hours.
	Expiry time.Duration

	// Required. Function returning a Redis connection (e.g. from a pool). Will
	// be closed when no longer needed.
	GetConn func() (redis.Conn, error)

	// Prefix to use for keys stored in Redis. It is recommended that this end in
	// "/".
	Prefix string
}

// Redis-backed session store.
type store struct {
	cfg Config
}

// Create a new redis-backed session store.
func New(cfg Config) (storage.Store, error) {
	s := &store{
		cfg: cfg,
	}

	if s.cfg.Expiry == 0 {
		s.cfg.Expiry = 4 * time.Hour
	}

	return s, nil
}

var ErrUnsupportedVersion = fmt.Errorf("unsupported serialization version")

// Returns key to store the given session ID at.
func (s *store) makeKey(sessionID storage.ID) string {
	return s.cfg.Prefix + uuid.FromBytesOrNil([]byte(sessionID)).String()
}

// Upsert set. If create is true, the session will be created if it does not
// exist. Otherwise, the session must already exist.
func (s *store) set(sessionID storage.ID, data map[string]interface{}, create bool) error {
	ms := &sess{
		Data:     data,
		LastSeen: time.Now(),
	}

	// Get connection from pool.
	conn, err := s.cfg.GetConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Serialize.
	buf := bytes.Buffer{}
	buf.WriteByte(0) // Version 0 serialization scheme.
	err = gob.NewEncoder(&buf).Encode(ms)
	log.Panice(err, "encode session") // should never happen

	// Assemble command.
	expiry := s.cfg.Expiry
	if lt, ok := data["session_lifetime"].(time.Duration); ok {
		expiry = lt
	}

	expirys := int(expiry.Seconds())
	args := redis.Args{}
	args = args.Add(s.makeKey(sessionID), buf.Bytes(), "EX", expirys)
	if !create {
		// Require key to already exist.
		args = args.Add("XX")
	}

	// Send command to Redis.
	_, err = conn.Do("SET", args...)
	log.Debuge(err, "set")

	return nil
}

// Get a session from Redis.
func (s *store) get(sessionID storage.ID) (*sess, error) {
	// Get connection from pool.
	conn, err := s.cfg.GetConn()
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	k := s.makeKey(sessionID)
	buf, err := redis.Bytes(conn.Do("GET", k))
	if err != nil {
		log.Debug("not found in redis: ", k)
		return nil, storage.ErrNotFound
	}

	// Check version.
	if len(buf) < 1 || buf[0] != 0 {
		return nil, ErrUnsupportedVersion
	}

	// Decode session.
	var v *sess
	err = gob.NewDecoder(bytes.NewReader(buf[1:])).Decode(&v)
	log.Panice(err, "bad session value")

	// Enforce expiry time even if Redis hasn't aged out the key yet.
	now := time.Now()
	if v.LastSeen.Add(s.cfg.Expiry).Before(now) {
		conn.Do("DEL", k) // best effort
		return nil, storage.ErrNotFound
	}

	// Touch.
	v.LastSeen = now
	return v, nil
}

// Create a new session.
func (s *store) Create() (sessionID storage.ID, err error) {
	u := uuid.NewV4()

	sessionID_ := storage.ID(u.Bytes())
	err = s.set(sessionID_, map[string]interface{}{}, true)
	if err != nil {
		return
	}

	sessionID = sessionID_
	return
}

func (s *store) Get(sessionID storage.ID) (x map[string]interface{}, err error) {
	v, err := s.get(sessionID)
	if err != nil {
		return nil, err
	}

	return v.Data, nil
}

// Set values for an existing session.
func (s *store) Set(sessionID storage.ID, x map[string]interface{}) error {
	return s.set(sessionID, x, false)
}

// Delete session.
func (s *store) Delete(sessionID storage.ID) error {
	// Get connection from pool.
	conn, err := s.cfg.GetConn()
	if err != nil {
		return err
	}

	defer conn.Close()

	// Delete session key.
	numDeleted, err := redis.Int(conn.Do("DEL", s.makeKey(sessionID)))
	if err != nil {
		return err
	}

	// Ensure a key was actually deleted.
	if numDeleted == 0 {
		return storage.ErrNotFound
	}

	return nil
}

func init() {
	// Make sure we can serialize time.Time.
	gob.Register(time.Time{})
	gob.Register(time.Duration(0))
}
