// Metrics nexus for Go.
package coremetric

import "sync"
import "fmt"

var UnregisteredMetricValue = ""

type MetricType int

const (
	// A generic metric supports only String() JSON representation.
	MetricTypeGeneric MetricType = iota

	// A gauge metric is an integral value which varies over time.
	MetricTypeGauge

	// A counter metric is a monotonously increasing integral value.
	MetricTypeCounter
)

type Metric interface {
	// Returns the name of the metric.
	Name() string

	// Returns the value of the metric as a string in JSON form.
	String() string

	// Returns the metric type.
	Type() MetricType

	// Returns integral value or 0 for non-integral types.
	Int64() int64
}

var metricsMutex sync.RWMutex
var metrics = map[string]Metric{}

// Visits all metrics. Because this read-locks the mutex storage, you must not
// call Register or Unregister from within this function.
func Do(f func(metric Metric)) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	for _, m := range metrics {
		f(m)
	}
}

// Registers a new metric. No metric with the same name must already exist.
func Register(metric Metric) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	metricName := metric.Name()
	_, exists := metrics[metricName]
	if exists {
		panic(fmt.Sprintf("A metric with the same name already exists: %s", metric.Name))
	}

	metrics[metricName] = metric
	callRegistrationHooks(metric, EventRegister)
}

// Unregister an existing metric. If no metric with the given name exists, does
// nothing.
func Unregister(metricName string) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	metric, ok := metrics[metricName]
	if !ok {
		return
	}

	callRegistrationHooks(metric, EventUnregister)
	delete(metrics, metricName)
}

func Get(metricName string) Metric {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	return metrics[metricName]
}

// Registration Hooks

type RegistrationHookEvent int

const (
	EventRegister RegistrationHookEvent = iota
	EventRegisterCatchup
	EventUnregister
)

type RegistrationHook func(metric Metric, event RegistrationHookEvent)

var registrationHooksMutex sync.RWMutex
var registrationHooks = map[interface{}]RegistrationHook{}

// Register for notifications on metric registration. The key must be usable as
// a key in a map and identifies the hook. No other hook with the same key must
// already exist.
//
// NOTE: The hook will be called for all registrations which already exist.
// This ensures that no registrations are missed in a threadsafe manner.
// For these calls, the event will be EventRegisterCatchup.
//
// The hook must not register or unregister registration hooks or metrics.
func RegisterHook(key interface{}, hook RegistrationHook) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	registerHook(key, hook)

	for _, m := range metrics {
		hook(m, EventRegisterCatchup)
	}
}

func registerHook(key interface{}, hook RegistrationHook) {
	registrationHooksMutex.Lock()
	defer registrationHooksMutex.Unlock()

	_, exists := registrationHooks[key]
	if exists {
		panic(fmt.Sprintf("A metric registration hook with the same key already exists: %+v", key))
	}

	registrationHooks[key] = hook
}

// Unregister an existing hook.
func UnregisterHook(key interface{}) {
	registrationHooksMutex.Lock()
	defer registrationHooksMutex.Unlock()
	delete(registrationHooks, key)
}

func callRegistrationHooks(metric Metric, event RegistrationHookEvent) {
	registrationHooksMutex.RLock()
	defer registrationHooksMutex.RUnlock()

	for _, v := range registrationHooks {
		v(metric, event)
	}
}
