// Redis backend. This can be used with memorysession as a fallback backend.
package redissession

import "code.google.com/p/go-uuid/uuid"
import "github.com/hlandau/degoutils/web/session/storage"
import "github.com/hlandau/xlog"
import "github.com/garyburd/redigo/redis"
import "encoding/gob"
import "bytes"
import "time"
import "fmt"

var log, Log = xlog.New("web.session.redissession")

type sess struct {
	Data     map[string]interface{}
	LastSeen time.Time
}

type Config struct {
	Expiry  time.Duration
	GetConn func() (redis.Conn, error)
	Prefix  string
}

type store struct {
	cfg Config
}

func New(cfg Config) (storage.Store, error) {
	s := &store{
		cfg: cfg,
	}

	if s.cfg.Expiry == 0 {
		s.cfg.Expiry = 4 * time.Hour
	}

	return s, nil
}

var ErrNotFound = fmt.Errorf("session not found")
var ErrUnsupportedVersion = fmt.Errorf("unsupported serialization version")

func (s *store) makeKey(sessionID storage.ID) string {
	return s.cfg.Prefix + uuid.UUID(sessionID).String()
}

func (s *store) set(sessionID storage.ID, data map[string]interface{}, create bool) error {
	ms := &sess{
		Data:     data,
		LastSeen: time.Now(),
	}

	conn, err := s.cfg.GetConn()
	if err != nil {
		return err
	}

	defer conn.Close()

	var buf bytes.Buffer
	buf.WriteByte(0) // v0
	err = gob.NewEncoder(&buf).Encode(ms)
	log.Panice(err, "encode session")

	expiry := int(s.cfg.Expiry.Seconds())
	var args redis.Args
	args = args.Add(s.makeKey(sessionID), buf.Bytes(), "EX", expiry)
	if !create {
		args = args.Add("XX")
	}

	_, err = conn.Do("SET", args...)
	if err != nil {
		log.Debug("redis set error: ", err, args)
	}

	return nil
}

func (s *store) get(sessionID storage.ID) (*sess, error) {
	conn, err := s.cfg.GetConn()
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	k := s.makeKey(sessionID)
	buf, err := redis.Bytes(conn.Do("GET", k))
	if err != nil {
		log.Debug("not found in redis: ", k)
		return nil, ErrNotFound
	}

	version := buf[0]
	if version != 0 {
		return nil, ErrUnsupportedVersion
	}

	var v *sess
	err = gob.NewDecoder(bytes.NewReader(buf[1:])).Decode(&v)
	log.Panice(err, "bad session value")

	now := time.Now()
	if v.LastSeen.Add(s.cfg.Expiry).Before(now) {
		conn.Do("DEL", k)
		return nil, ErrNotFound
	}

	v.LastSeen = now
	return v, nil
}

func (s *store) Create() (sessionID storage.ID, err error) {
	u := uuid.NewUUID()
	if u == nil {
		log.Panic("cannot create UUID")
	}

	sessionID_ := storage.ID([]byte(u))
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

func (s *store) Set(sessionID storage.ID, x map[string]interface{}) error {
	return s.set(sessionID, x, false)
}

func (s *store) Delete(sessionID storage.ID) error {
	conn, err := s.cfg.GetConn()
	if err != nil {
		return err
	}

	defer conn.Close()

	numDeleted, err := redis.Int(conn.Do("DEL", s.makeKey(sessionID)))
	if err != nil {
		return err
	}

	if numDeleted == 0 {
		return ErrNotFound
	}

	return nil
}

func init() {
	gob.Register(time.Time{})
}
