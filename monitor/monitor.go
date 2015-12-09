// Package monitor provides utilities for launching goroutines and being
// informed when they exit.
package monitor

type EventType int

const (
	NormalExit  EventType = iota // The goroutine exited normally.
	PanicExit                    // The goroutine panicked.
	RuntimeExit                  // The goroutine was terminated via runtime.Goexit().
)

// A goroutine monitoring event.
type Event struct {
	Type  EventType
	Panic interface{} // If the goroutine panicked, this is the panic value.
	Error error       // The value returned by the function.
}

// Runs a function in a goroutine. When the goroutine exits, send a single
// event on the returned channel.
func Monitor(f func() error) <-chan Event {
	ch := make(chan Event, 1)

	go func() {
		normalExit := false
		var err error
		defer func() {
			r := recover()
			if r != nil {
				ch <- Event{PanicExit, r, nil}
			} else if normalExit {
				ch <- Event{NormalExit, nil, err}
			} else {
				ch <- Event{RuntimeExit, nil, nil}
			}
		}()

		err = f()
		normalExit = true
	}()

	return ch
}
