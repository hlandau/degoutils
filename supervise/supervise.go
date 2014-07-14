// WORK IN PROGRESS
package supervise

const (
  MET_NormalExit  = 0
  MET_PanicExit   = 1
  MET_RuntimeExit = 2
)

type MonitorEvent struct {
  Type int
  PanicValue interface{}
}

func Monitor(f func()) chan MonitorEvent {
  ch := make(chan MonitorEvent, 10)
  go func() {
    var normalExit bool = false
    defer func() {
      r := recover()
      if r != nil {
        // recover ...
        ch <- MonitorEvent{MET_PanicExit, r}
      } else if normalExit {
        ch <- MonitorEvent{MET_NormalExit, nil}
      } else {
        ch <- MonitorEvent{MET_RuntimeExit, nil}
      }
    }()

    f()

    normalExit = true
  }()
  return ch
}

////////////////////

const (
  SCT_StopSupervising = 1
)

type SupervisionCommand struct {
  Type int
}

func Supervise(f func()) {
  cch := make(chan SupervisionCommand)
  go func() {
    ch := Monitor(f)
    for {
      select {
        case e  := <-ch:
          if e.Type != MET_NormalExit {
            ch = Monitor(f)
          }

        case ce := <-cch:
          switch ce.Type {
            case SCT_StopSupervising:
              return
          }
      }
    }
  }()
}

func unused(x interface{}) {}

func main() {
  ch := Monitor(func() {
    // ...
  })
  unused(ch)
}
