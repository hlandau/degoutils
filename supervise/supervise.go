// WORK IN PROGRESS
package supervise

import "github.com/hlandau/degoutils/log"
import "github.com/hlandau/degoutils/net"
import "fmt"
import "time"

const (
	MET_NormalExit  = 0
	MET_PanicExit   = 1
	MET_RuntimeExit = 2
)

type MonitorEvent struct {
	Type        int
	PanicValue  interface{}
	ReturnError error
}

func Monitor(f func() error) <-chan MonitorEvent {
	ch := make(chan MonitorEvent, 10)
	go func() {
		var normalExit bool = false
		var err error
		defer func() {
			r := recover()
			if r != nil {
				// recover ...
				ch <- MonitorEvent{MET_PanicExit, r, nil}
			} else if normalExit {
				ch <- MonitorEvent{MET_NormalExit, nil, err}
			} else {
				ch <- MonitorEvent{MET_RuntimeExit, nil, nil}
			}
		}()

		err = f()

		normalExit = true
	}()
	return ch
}

////////////////////

type Supervisor interface {
	EventChan() <-chan SupervisionEvent
	Stop()
}

const (
	SET_NormalExit = 1
	SET_Stopped    = 2
)

type SupervisionEvent struct {
	Type int
}

const (
	SCT_StopSupervising = 1
)

type SupervisionCommand struct {
	Type int
}

type supervisor struct {
	cch         chan SupervisionCommand
	evch        chan SupervisionEvent
	retryConfig net.Backoff
}

func (s *supervisor) Stop() {
	s.cch <- SupervisionCommand{SCT_StopSupervising}
}

func (s *supervisor) EventChan() <-chan SupervisionEvent {
	return s.evch
}

func Supervise(f func() error) Supervisor {
	sup := &supervisor{
		cch:  make(chan SupervisionCommand),
		evch: make(chan SupervisionEvent, 10),
	}

	go func() {
		ch := Monitor(f)
		for {
			select {
			case e := <-ch:
				if e.Type != MET_NormalExit || e.ReturnError != nil {
					delay := time.Duration(sup.retryConfig.GetStepDelay()) * time.Millisecond
					log.Info(fmt.Sprintf("supervised goroutine exited, restarting in %+v: %+v", delay, e))
					time.Sleep(delay)
					ch = Monitor(f)
				} else {
					sup.evch <- SupervisionEvent{SET_NormalExit}
				}

			case ce := <-sup.cch:
				switch ce.Type {
				case SCT_StopSupervising:
					close(sup.cch)
					sup.evch <- SupervisionEvent{SET_Stopped}
					return
				}
			}
		}
	}()

	return sup
}
