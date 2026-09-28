package driver

import (
	"context"
	"fmt"
	"reflect"
)

type Composite struct{ ports []Port }

func NewComposite(ports ...Port) *Composite {
	c := &Composite{}
	for _, p := range ports {
		if isNil(p) {
			continue
		}
		c.ports = append(c.ports, p)
	}
	return c
}

func (c *Composite) Start(ctx context.Context) {}

func (c *Composite) Write(ctx context.Context, tag string, value any) error {
	for _, p := range c.ports {
		if p.HasTag(tag) {
			return p.Write(ctx, tag, value)
		}
	}
	return fmt.Errorf("tag not found: %s", tag)
}

func (c *Composite) HasTag(tag string) bool {
	for _, p := range c.ports {
		if p.HasTag(tag) {
			return true
		}
	}
	return false
}

func (c *Composite) DeviceStatuses() []DeviceStatus {
	var out []DeviceStatus
	for _, p := range c.ports {
		out = append(out, p.DeviceStatuses()...)
	}
	return out
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func:
		return rv.IsNil()
	}
	return false
}
