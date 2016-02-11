// Package memorysession provides an in-memory session store. Can optionally be
// used with a fallback backend, in which case it acts as a sort of cache.
package memorysession

import "github.com/satori/go.uuid"
import "github.com/hlandau/degoutils/web/session/storage"
import "github.com/hlandau/xlog"
import "time"
import "sync"

var log, Log = xlog.New("web.session.memorysession")

type sess struct {
	data     map[string]interface{}
	lastSeen time.Time
}

// Memory-based session store configuration.
type Config struct {
	// After what period of inactivity should sessions be removed from the store?
	// Sessions will still be left in any fallback store which is configured.
	//
	// Defaults to 4 hours.
	Expiry time.Duration

	// If set, acts as a writeback cache. If a session is not found in memory, it
	// is looked for in the fallback store. All session writes are persisted to
	// the fallback store.
	FallbackStore storage.Store
}

// Memory-based session store.
type store struct {
	storeMutex sync.Mutex
	store      map[storage.ID]*sess

	cfg Config
}

// Create a memory-based session store.
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

func (s *store) cleanupLoop() {
	for {
		time.Sleep(1 * time.Minute)
		s.doCleanup()
	}
}

// Remove expired sessions from the memory store (but not any fallback store).
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

// Set a session in the memory store, creating it if it doesn't exist. The
// session is touched. Lock must be held.
func (s *store) lCreatingSet(sessionID storage.ID, v map[string]interface{}) {
	ms := &sess{
		data:     v,
		lastSeen: time.Now(),
	}

	s.store[sessionID] = ms
}

func (s *store) lockingCreatingSet(sessionID storage.ID, v map[string]interface{}) {
	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()
	s.lCreatingSet(sessionID, v)
}

// Create a new session and return the ID.
func (s *store) Create() (sessionID storage.ID, err error) {
	if s.cfg.FallbackStore != nil {
		sessionID, err = s.cfg.FallbackStore.Create()
	}

	// resilience: go from memory on fallback store failure
	if s.cfg.FallbackStore == nil || err != nil {
		u := uuid.NewV4()
		if u == nil {
			log.Panic("cannot create UUID")
		}

		sessionID = storage.ID(u.Bytes())
	}

	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()
	s.lCreatingSet(sessionID, map[string]interface{}{})
	return
}

func (s *store) lGet(sessionID storage.ID) (*sess, error) {
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

func (s *store) lockingGet(sessionID storage.ID) (map[string]interface{}, error) {
	s.storeMutex.Lock()
	defer s.storeMutex.Unlock()
	v, err := s.lGet(sessionID)
	if err != nil {
		return nil, err
	}

	return v.data, nil
}

func (s *store) Get(sessionID storage.ID) (x map[string]interface{}, err error) {
	v, err := s.lockingGet(sessionID)
	if err != nil {
		// Not found in memory store, see if it's in the fallback store.
		// If it is, cache it in the memory store and return it.
		if err == storage.ErrNotFound && s.cfg.FallbackStore != nil {
			data, err := s.cfg.FallbackStore.Get(sessionID)
			if err == nil {
				s.lockingCreatingSet(sessionID, data)
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

	v, err := s.lGet(sessionID)
	if err != nil {
		return err
	}

	v.data = x

	// Writeback.
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
