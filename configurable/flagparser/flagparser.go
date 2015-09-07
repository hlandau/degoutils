package flagparser

import "fmt"
import "github.com/hlandau/degoutils/configurable"

func name(c configurable.Configurable) (name string, ok bool) {
	v, ok := c.(interface {
		CfName() string
	})
	if !ok {
		return
	}

	return v.CfName(), true
}

func usageSummaryLine(c configurable.Configurable) (s string, ok bool) {
	v, ok := c.(interface {
		CfUsageSummaryLine() string
	})
	if !ok {
		return
	}

	return v.CfUsageSummaryLine(), true
}

func defaultValue(c configurable.Configurable) (s interface{}, ok bool) {
	v, ok := c.(interface {
		CfDefaultValue() interface{}
	})
	if !ok {
		return
	}

	return v.CfDefaultValue(), true
}

func defaultValueString(c configurable.Configurable) string {
	s, ok := defaultValue(c)
	if !ok {
		return ""
	}

	return fmt.Sprintf("%v", s)
}

var errNotSupported = fmt.Errorf("not supported")

func printFlagUsage(c configurable.Configurable) error {
	name, ok := name(c)
	if !ok {
		return errNotSupported
	}

	_, ok = c.(interface {
		CfSetValue(v interface{}) error
	})
	if !ok {
		return errNotSupported
	}

	usage, _ := usageSummaryLine(c)
	defValue := defaultValueString(c)
	col1 := fmt.Sprintf("  -%s=%s  ", name, defValue)

	fmt.Printf("%-40s%s\n", col1, usage)

	return nil
}

func processConfigurable(c configurable.Configurable) error {
	err := printFlagUsage(c)
	if err != nil {
		fmt.Printf("%v\n", c)
	}

	for _, ch := range c.CfChildren() {
		processConfigurable(ch)
	}

	return nil
}

func ParseFlags() {
	configurable.Visit(func(c configurable.Configurable) error {
		return processConfigurable(c)
	})
}
