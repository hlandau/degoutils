// Package health provides a simple web server health monitoring system.
//
// A series of criterions are created. The process is considered to be
// in good health if all criterions have positive counts. Otherwise,
// the process is in bad health. The internal implementation uses refcounting.
//
// The health of the process can be queried at /health on the default
// HTTP serve mux. This returns 200 or 503. /health/info provides more detailed
// info about bad criterions.
package health

import "net/http"
import "sync"
import "sync/atomic"
import "fmt"
import "bytes"

var badCriteriaCount uint64
var badCriteria = map[*Criterion]struct{}{}
var badCriteriaMutex sync.RWMutex

type Criterion struct {
	name   string
	status string
	value  int64
}

// Create a new criterion. If ok is true, the initial counter value
// is 1; otherwise, it is 0.
func NewCriterion(name string, ok bool) *Criterion {
	c := &Criterion{
		name:  name,
		value: 1,
	}
	if !ok {
		c.Dec()
	}
	return c
}

// A descriptive string representing the criterion.
func (c *Criterion) String() string {
	return fmt.Sprintf("criterion \"%s\": %v: %s", c.name, c.value, c.status)
}

// Add to the criterion counter. If the resulting count is positive,
// the criterion is in good health.
func (c *Criterion) Add(x int) {
	oldValue := atomic.LoadInt64(&c.value)
	newValue := oldValue + int64(x)
	oldValueIsBad := oldValue <= 0
	newValueIsBad := newValue <= 0

	if !atomic.CompareAndSwapInt64(&c.value, oldValue, newValue) {
		c.Add(x)
	} else if oldValueIsBad != newValueIsBad {
		badCriteriaMutex.Lock()
		defer badCriteriaMutex.Unlock()

		if newValueIsBad {
			// gone bad
			badCriteria[c] = struct{}{}
			atomic.AddUint64(&badCriteriaCount, 1)
		} else {
			// gone good
			delete(badCriteria, c)
			atomic.AddUint64(&badCriteriaCount, ^uint64(0)) // decrement
		}
	}
}

// Subtract from the criterion counter. If the resulting count is not
// positive, the criterion is in bad healt.
func (c *Criterion) Sub(x int) {
	c.Add(-x)
}

// Increment the counter.
func (c *Criterion) Inc() {
	c.Add(1)
}

// Decrement the counter.
func (c *Criterion) Dec() {
	c.Sub(1)
}

// Returns the criterion name passed at creation.
func (c *Criterion) Name() string {
	return c.name
}

// Set the criterion status. This is a freeform string which
// you may optionally use to describe the current criterion status.
func (c *Criterion) SetStatus(status string) {
	c.status = status
}

// Return the current criterion status. The default status is the empty
// string.
func (c *Criterion) Status() string {
	return c.status
}

// Return the criterion counter. If the counter is positive, the criterion
// is in good health.
func (c *Criterion) Value() int {
	return int(c.value)
}

func init() {
	http.HandleFunc("/health", handler)
	http.HandleFunc("/health/info", detailedHandler)
}

var okResponse = []byte{'O', 'K'}
var errResponse = []byte{'E', 'R', 'R'}

func handler(rw http.ResponseWriter, req *http.Request) {
	bc := atomic.LoadUint64(&badCriteriaCount)
	if bc > 0 {
		rw.WriteHeader(503)
		rw.Write(errResponse)
	} else {
		rw.Write(okResponse)
	}
}

func detailedHandler(rw http.ResponseWriter, req *http.Request) {
	badCriteriaMutex.RLock()
	defer badCriteriaMutex.RUnlock()

	var buf bytes.Buffer
	if len(badCriteria) > 0 {
		rw.WriteHeader(503)
		fmt.Fprintf(&buf, "ERR %v\n", len(badCriteria))
	} else {
		buf.WriteString("OK\n")
	}

	for c := range badCriteria {
		fmt.Fprintf(&buf, "%s\n", c.String())
	}
	rw.Write(buf.Bytes())
}
