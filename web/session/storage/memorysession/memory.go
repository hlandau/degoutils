// In-memory storage. Can be used with a fallback backend, in which case it
// acts as a sort of cache.
package memorysession

import "code.google.com/p/go-uuid/uuid"
import "github.com/hlandau/degoutils/web/session/storage"
import "github.com/hlandau/xlog"
import "time"
import "sync"

var log, Log = xlog.New("web.session.memorysession")

type sess struct {
	data     map[string]interface{}
	lastSeen time.Time
}

type Config struct {
	Expiry        time.Duration
	FallbackStore storage.Store
}

type store struct {
	storeMutex sync.Mutex
	store      map[storage.ID]*sess

	cfg Config
}

func (s *store) doCleanup() {
	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()

	var toDelete []storage.ID
	now := time.Now()
	for id, v := range s.store {
		if v.lastSeen.Add(s.cfg.Expiry).Before(now) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(s.store, id)
	}
}

func (s *store) cleanupLoop() {
	for {
		time.Sleep(1 * time.Minute)
		s.doCleanup()
	}
}

func New(cfg Config) (storage.Store, error) {
	s := &store{
		cfg:   cfg,
		store: map[storage.ID]*sess{},
	}

	if s.cfg.Expiry == 0 {
		s.cfg.Expiry = 4 * time.Hour
	}

	go s.cleanupLoop()

	return s, nil
}

func (s *store) creatingSet(sessionID storage.ID, v map[string]interface{}) {
	ms := &sess{
		data:     v,
		lastSeen: time.Now(),
	}

	s.store[sessionID] = ms
}

func (s *store) Create() (sessionID storage.ID, err error) {
	if s.cfg.FallbackStore != nil {
		sessionID, err = s.cfg.FallbackStore.Create()
	}

	// resilience: go from memory on fallback store failure
	if s.cfg.FallbackStore == nil || err != nil {
		u := uuid.NewUUID()
		if u == nil {
			log.Panic("cannot create UUID")
		}

		sessionID = storage.ID([]byte(u))
	}

	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()
	s.creatingSet(sessionID, map[string]interface{}{})
	return
}

func (s *store) get(sessionID storage.ID) (*sess, error) {
	v, ok := s.store[sessionID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	now := time.Now()
	if v.lastSeen.Add(s.cfg.Expiry).Before(now) {
		delete(s.store, sessionID)
		return nil, storage.ErrNotFound
	}

	v.lastSeen = now
	return v, nil
}

func (s *store) lget(sessionID storage.ID) (map[string]interface{}, error) {
	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()
	v, err := s.get(sessionID)
	if err != nil {
		return nil, err
	}

	return v.data, nil
}

func (s *store) Get(sessionID storage.ID) (x map[string]interface{}, err error) {
	v, err := s.lget(sessionID)
	if err != nil {
		if err == storage.ErrNotFound && s.cfg.FallbackStore != nil {
			data, err := s.cfg.FallbackStore.Get(sessionID)
			if err == nil {
				s.creatingSet(sessionID, data)
				return data, nil
			}
		}
		return nil, err
	}

	return v, nil
}

func (s *store) Set(sessionID storage.ID, x map[string]interface{}) error {
	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()

	v, err := s.get(sessionID)
	if err != nil {
		return err
	}

	v.data = x

	if s.cfg.FallbackStore != nil {
		err := s.cfg.FallbackStore.Set(sessionID, x)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *store) Delete(sessionID storage.ID) error {
	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()

	_, ok := s.store[sessionID]
	if !ok {
		return storage.ErrNotFound
	}

	delete(s.store, sessionID)

	if s.cfg.FallbackStore != nil {
		s.cfg.FallbackStore.Delete(sessionID)
		// ignore errors
	}

	return nil
}
