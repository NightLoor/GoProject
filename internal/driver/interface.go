package driver

import (
	"context"
	"time"
)

type DeviceStatus struct {
	Name          string
	Protocol      string
	Address       string
	Connected     bool
	LastConnected time.Time
	LastPoll      time.Time
	Error         string
}

type Port interface {
	Start(context.Context)
	Write(context.Context, string, any) error
	HasTag(string) bool
	DeviceStatuses() []DeviceStatus
}
