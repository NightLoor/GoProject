//go:build !windows

package opcda

import (
	"context"
	"fmt"
	"time"

	"modbus-tcp-driver-v3/internal/driver"
	"modbus-tcp-driver-v3/internal/io"
	"modbus-tcp-driver-v3/internal/model"
)

type Driver struct{}

func NewDriver(_ model.Config, _ *io.Manager) (*Driver, error) { return &Driver{}, nil }
func (d *Driver) Start(_ context.Context)                      {}
func (d *Driver) Write(_ context.Context, tag string, _ any) error {
	return fmt.Errorf("OPC DA driver requires Windows/COM: tag %s", tag)
}
func (d *Driver) HasTag(_ string) bool                  { return false }
func (d *Driver) DeviceStatuses() []driver.DeviceStatus { return nil }

var _ = time.Second
