package expvaradaptor

import "github.com/hlandau/degoutils/metric/coremetric"
import "sync"
import "expvar"

var UnregisteredMetricValue = "null"

var once sync.Once
var hookKey int

type expvarAdaptor struct {
	Mutex  sync.RWMutex
	Metric coremetric.Metric
}

var metricsMutex sync.RWMutex
var metrics = map[string]*expvarAdaptor{}

func (ea *expvarAdaptor) String() string {
	ea.Mutex.RLock()
	m := ea.Metric
	ea.Mutex.RUnlock()

	if m == nil {
		return UnregisteredMetricValue
	}

	return m.String()
}

func hook(m coremetric.Metric, event coremetric.RegistrationHookEvent) {
	metricName := m.Name()

	switch event {
	case coremetric.EventRegister, coremetric.EventRegisterCatchup:
		expvarAdaptor := &expvarAdaptor{
			Metric: m,
		}

		metricsMutex.Lock()
		defer metricsMutex.Unlock()

		metrics[metricName] = expvarAdaptor
		expvar.Publish(metricName, expvarAdaptor)

	case coremetric.EventUnregister:
		expvarAdaptor := metrics[metricName]
		if expvarAdaptor != nil { // this should always be the case, but whatever
			expvarAdaptor.Mutex.Lock()
			defer expvarAdaptor.Mutex.Unlock()

			expvarAdaptor.Metric = nil
		}
	}
}

func Register() {
	once.Do(func() {
		coremetric.RegisterHook(&hookKey, hook)
	})
}
