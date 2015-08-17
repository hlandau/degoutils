package metric

import "github.com/hlandau/degoutils/metric/coremetric"
import "github.com/hlandau/degoutils/metric/expvaradaptor"
import "github.com/hlandau/degoutils/metric/prometheusadaptor"
import "sync/atomic"
import "strconv"

type Counter struct {
	name  string
	value int64
}

func (c *Counter) Name() string {
	return c.name
}

func (c *Counter) String() string {
	v := atomic.LoadInt64(&c.value)
	return strconv.FormatInt(v, 10)
}

func (c *Counter) Add(v int64) {
	atomic.AddInt64(&c.value, v)
}

func (c *Counter) Inc() {
	c.Add(1)
}

func (c *Counter) Type() coremetric.MetricType {
	return coremetric.MetricTypeCounter
}

func (c *Counter) Int64() int64 {
	return atomic.LoadInt64(&c.value)
}

func NewCounter(name string) *Counter {
	c := &Counter{
		name: name,
	}

	coremetric.Register(c)
	return c
}

func RegisterAdaptors() {
	expvaradaptor.Register()
	prometheusadaptor.Register()
}
