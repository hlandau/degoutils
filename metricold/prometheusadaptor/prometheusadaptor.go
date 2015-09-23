package prometheusadaptor

import "github.com/hlandau/degoutils/metric/coremetric"
import "github.com/prometheus/client_golang/prometheus"

//import "github.com/prometheus/client_model/go"
import "sync"
import "regexp"
import "errors"
import "net/http"

type metric struct {
	Metric    coremetric.Metric
	ProMetric prometheus.Metric
}

var errNotSupported = errors.New("not supported")

func (m *metric) init() error {
	metricName := m.Metric.Name()
	mangledName := mangleName(metricName)

	opts := prometheus.Opts{
		Name: mangledName,
		Help: metricName,
	}

	switch m.Metric.Type() {
	case coremetric.MetricTypeCounter:
		m.ProMetric = prometheus.NewCounterFunc(prometheus.CounterOpts(opts), func() float64 {
			return float64(m.Metric.Int64())
		})

	case coremetric.MetricTypeGauge:
		m.ProMetric = prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
			return float64(m.Metric.Int64())
		})

	default:
		return errNotSupported
	}

	return nil
}

type collector struct{}

var metricsMutex sync.RWMutex
var metrics = map[string]*metric{}

func (c *collector) Describe(descChan chan<- *prometheus.Desc) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	for _, m := range metrics {
		descChan <- m.ProMetric.Desc()
	}
}

func (c *collector) Collect(metricChan chan<- prometheus.Metric) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	for _, m := range metrics {
		metricChan <- m.ProMetric
	}
}

var col collector

var re_mangler = regexp.MustCompilePOSIX(`[^a-zA-Z0-9_:]`)

func mangleName(metricName string) string {
	return re_mangler.ReplaceAllString(metricName, "_")
}

func hook(m coremetric.Metric, event coremetric.RegistrationHookEvent) {
	metricName := m.Name()

	switch event {
	case coremetric.EventRegister, coremetric.EventRegisterCatchup:
		mi := &metric{
			Metric: m,
		}
		err := mi.init()
		if err != nil {
			return
		}

		metricsMutex.Lock()
		defer metricsMutex.Unlock()
		metrics[metricName] = mi

	case coremetric.EventUnregister:
		metricsMutex.Lock()
		defer metricsMutex.Unlock()
		delete(metrics, metricName)
	}
}

var once, handlerOnce sync.Once
var hookKey int

func RegisterNoNexus() {
	once.Do(func() {
		coremetric.RegisterHook(&hookKey, hook)
		prometheus.Register(&col)
	})
}

func Register() {
	RegisterNoNexus()
	handlerOnce.Do(func() {
		http.Handle("/metrics", prometheus.Handler())
	})
}
