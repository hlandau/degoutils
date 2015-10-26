// Session storage, for use with session-pinning webservers only.
//
// This session system has no locking and is suitable only for use
// on a single replica, or behind a load balancer which performs
// session pinning.
package storage

import "fmt"

type ID string

// Represents a session store.
type Store interface {
	// Create a session. Returns a unique session ID.
	Create() (ID, error)

	// Get a session by ID. Returns error if the session does not exist.
	Get(ID) (map[string]interface{}, error)

	// Set a session. Returns error if the session does not exist.
	Set(ID, map[string]interface{}) error

	// Delete a session. Returns error if the session does not exist.
	Delete(ID) error
}

var ErrNotFound = fmt.Errorf("session not found")
