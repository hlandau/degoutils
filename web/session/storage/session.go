// Session storage, for use with session-pinning webservers only.
//
// This session system has no locking and is suitable only for use
// on a single replica, or behind a load balancer which performs
// session pinning.
package storage

import "fmt"

// A session ID. Should be treated as an opaque identifying string.
//
// Must be guaranteed to be unique (e.g. a UUID or monotonically incrementing
// integer).
type ID string

// Represents a session store.
type Store interface {
	// Create a session. Returns a unique session ID.
	//
	// In order to make this interface idiotproof in relation to session
	// fixation, sessions can only be created via this method, not via Set.
	Create() (ID, error)

	// Get a session by ID. Returns error if the session does not exist.
	//
	// The consuming code may mutate the returned map, but must do so only if it
	// guarantees that it will later call Set with the same ID and that same map.
	// Such changes may manifest in future calls to Get even before the call to
	// Set or even if Set is not called; i.e., for memory-based session stores,
	// this may be the map used internally, not a copy.
	//
	// The session must have been created via a call to Create.
	Get(ID) (map[string]interface{}, error)

	// Set a session. Returns error if the session does not exist.
	//
	// The session must have been created via a call to Create.
	Set(ID, map[string]interface{}) error

	// Delete a session. Returns error if the session does not exist.
	Delete(ID) error
}

// Error returned if the session with the given ID is not found.
var ErrNotFound = fmt.Errorf("session not found")
