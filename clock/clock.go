package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
	After(time.Duration) <-chan time.Time
}

var Real Clock

func init() {
	Real = realClock{}
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func (realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type Fake interface {
	Clock
	Advance(time.Duration)
}

// A fast fake clock returns from Sleep calls immediately.
//
// Any waiting operation appears to complete immediately, as though time is
// running infinitely fast, but only when waiting.
func NewFast(from Clock) Fake {
	if from == nil {
		from = Real
	}

	return NewFastAt(from.Now())
}

func NewFastAt(t time.Time) Fake {
	return &fastFake{t: t}
}

type fastFake struct {
	t     time.Time
	mutex sync.RWMutex
}

func (f *fastFake) Now() time.Time {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	t := f.t
	return t
}

func (f *fastFake) Sleep(d time.Duration) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.t = f.t.Add(d)
}

func (f *fastFake) Advance(d time.Duration) {
	f.Sleep(d)
}

func (f *fastFake) After(d time.Duration) <-chan time.Time {
	f.Sleep(d)
	c := make(chan time.Time, 1)
	c <- f.Now()
	return c
}

// A slow clock doesn't return from Sleep calls until Advance has been called
// enough.
func NewSlow(from Clock) Fake {
	if from == nil {
		from = Real
	}

	return NewSlowAt(from.Now())
}

func NewSlowAt(t time.Time) Fake {
	return &slowFake{t: t}
}

type slowFake struct {
	t        time.Time
	mutex    sync.RWMutex
	sleepers []*slowSleeper
}

type slowSleeper struct {
	until time.Time
	done  chan<- time.Time
}

func (f *slowFake) Now() time.Time {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	t := f.t
	return t
}

func (f *slowFake) Sleep(d time.Duration) {
	<-f.After(d)
}

func (f *slowFake) Advance(d time.Duration) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	t2 := f.t.Add(d)
	var newSleepers []*slowSleeper
	for _, s := range f.sleepers {
		if t2.Sub(s.until) >= 0 {
			s.done <- t2
		} else {
			newSleepers = append(newSleepers, s)
		}
	}

	f.sleepers = newSleepers
	f.t = t2
}

func (f *slowFake) After(d time.Duration) <-chan time.Time {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	done := make(chan time.Time, 1)
	if d == 0 {
		done <- f.t
		return done
	}

	s := &slowSleeper{
		until: f.t.Add(d),
		done:  done,
	}

	f.sleepers = append(f.sleepers, s)
	return done
}

// © 2015 Jonathan Boulle   Apache 2.0 License
