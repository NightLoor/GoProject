//go:build windows

package opcda

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/huskar-t/opcda"

	"modbus-tcp-driver-v3/internal/driver"
	"modbus-tcp-driver-v3/internal/io"
	"modbus-tcp-driver-v3/internal/model"
)

type Tag struct {
	Config model.OPCDATagConfig
	Item   *opcda.OPCItem
}

type Device struct {
	cfg       model.OPCDADeviceConfig
	io        *io.Manager
	server    *opcda.OPCServer
	group     *opcda.OPCGroup
	tags      []Tag
	index     map[string]Tag
	reconnect time.Duration
	poll      time.Duration
	status    *statusStore
	mu        sync.Mutex
}

type Driver struct {
	devices []*Device
	poll    time.Duration
	status  *statusStore
}

type statusStore struct {
	mu     sync.RWMutex
	values map[string]driver.DeviceStatus
}

func newStatusStore() *statusStore { return &statusStore{values: make(map[string]driver.DeviceStatus)} }
func (s *statusStore) set(name string, fn func(*driver.DeviceStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.values[name]
	fn(&v)
	s.values[name] = v
}
func (s *statusStore) all() []driver.DeviceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]driver.DeviceStatus, 0, len(s.values))
	for _, v := range s.values {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func NewDriver(cfg model.Config, ioManager *io.Manager) (*Driver, error) {
	if !cfg.IsOPCDAEnabled() {
		return &Driver{status: newStatusStore()}, nil
	}
	poll := time.Duration(cfg.PollIntervalMS) * time.Millisecond
	if poll <= 0 {
		poll = time.Second
	}
	d := &Driver{poll: poll, status: newStatusStore()}
	for _, dc := range cfg.OPCDADevices {
		if dc.ProgID == "" {
			return nil, fmt.Errorf("opc da device %s prog_id is empty", dc.Name)
		}
		if dc.Node == "" {
			dc.Node = "localhost"
		}
		if dc.GroupName == "" {
			dc.GroupName = "GoMonitor"
		}
		dev := &Device{
			cfg: dc, io: ioManager, index: make(map[string]Tag),
			reconnect: durationOr(dc.ReconnectIntervalMS, 5000), poll: poll, status: d.status,
		}
		for _, tc := range dc.Tags {
			if tc.Name == "" || tc.ItemID == "" {
				return nil, fmt.Errorf("opc da device=%s tag name/item_id is empty", dc.Name)
			}
			if tc.Scale == 0 {
				tc.Scale = 1
			}
			if _, exists := dev.index[tc.Name]; exists {
				return nil, fmt.Errorf("duplicate OPC DA tag %s on device %s", tc.Name, dc.Name)
			}
			dev.tags = append(dev.tags, Tag{Config: tc})
			dev.index[tc.Name] = dev.tags[len(dev.tags)-1]
		}
		d.devices = append(d.devices, dev)
		d.status.set(dc.Name, func(st *driver.DeviceStatus) {
			st.Name, st.Protocol, st.Address = dc.Name, "OPC DA", fmt.Sprintf("%s@%s", dc.ProgID, dc.Node)
		})
	}
	return d, nil
}

func durationOr(ms, fallback int) time.Duration {
	if ms <= 0 {
		ms = fallback
	}
	return time.Duration(ms) * time.Millisecond
}
func (d *Driver) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for _, dev := range d.devices {
		wg.Add(1)
		go func(x *Device) { defer wg.Done(); x.run(ctx) }(dev)
	}
	wg.Wait()
}
func (d *Driver) HasTag(name string) bool {
	for _, dev := range d.devices {
		if _, ok := dev.index[name]; ok {
			return true
		}
	}
	return false
}
func (d *Driver) Write(ctx context.Context, name string, value any) error {
	for _, dev := range d.devices {
		if tag, ok := dev.index[name]; ok {
			return dev.write(ctx, tag.Config, value)
		}
	}
	return fmt.Errorf("tag not found: %s", name)
}
func (d *Driver) DeviceStatuses() []driver.DeviceStatus { return d.status.all() }

var (
	ole32              = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procCoUninitialize = ole32.NewProc("CoUninitialize")
)

const coinitializeMultithreaded = 0x0

func initCOM() error {
	r1, _, _ := procCoInitializeEx.Call(0, coinitializeMultithreaded)
	// S_OK (0) and S_FALSE (1) both mean the thread is COM-initialized.
	if r1 == 0 || r1 == 1 {
		return nil
	}
	return fmt.Errorf("CoInitializeEx failed: HRESULT=0x%08X", uint32(r1))
}

func uninitCOM() {
	procCoUninitialize.Call()
}

func (dev *Device) run(ctx context.Context) {
	// OPC DA is COM/DCOM based. COM apartment initialization is thread-local,
	// so keep the entire device worker on one OS thread for its lifetime.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initCOM(); err != nil {
		log.Printf("opcda COM initialize failed: device=%s err=%v", dev.cfg.Name, err)
		dev.setBad(err)
		return
	}
	defer uninitCOM()

	for {
		if err := dev.connect(); err != nil {
			log.Printf("opcda connect failed: device=%s prog_id=%s node=%s err=%v", dev.cfg.Name, dev.cfg.ProgID, dev.cfg.Node, err)
			dev.setBad(err)
			if !waitOrCancel(ctx, dev.reconnect) {
				return
			}
			continue
		}
		dev.setConnected()
		log.Printf("opcda device connected: %s (%s@%s)", dev.cfg.Name, dev.cfg.ProgID, dev.cfg.Node)
		if err := dev.pollOnce(ctx); err != nil {
			log.Printf("opcda device poll error: %s: %v", dev.cfg.Name, err)
			dev.close()
			dev.setBad(err)
			if !waitOrCancel(ctx, dev.reconnect) {
				return
			}
			continue
		}
		ticker := time.NewTicker(dev.poll)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				dev.close()
				return
			case <-ticker.C:
				if err := dev.pollOnce(ctx); err != nil {
					ticker.Stop()
					dev.close()
					dev.setBad(err)
					goto reconnect
				}
			}
		}
	reconnect:
		if !waitOrCancel(ctx, dev.reconnect) {
			return
		}
	}
}

func waitOrCancel(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (dev *Device) connect() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.server != nil {
		_ = dev.server.Disconnect()
		dev.server = nil
	}
	server, err := opcda.Connect(dev.cfg.ProgID, dev.cfg.Node)
	if err != nil {
		return fmt.Errorf("opc da Connect(%q,%q): %w", dev.cfg.ProgID, dev.cfg.Node, err)
	}
	if err := server.SetClientName("GoModbusOPCDA"); err != nil {
		_ = server.Disconnect()
		return fmt.Errorf("set client name: %w", err)
	}
	groups := server.GetOPCGroups()
	group, err := groups.Add(dev.cfg.GroupName)
	if err != nil {
		_ = groups.Release()
		_ = server.Disconnect()
		return fmt.Errorf("add OPC group %q: %w", dev.cfg.GroupName, err)
	}
	_ = group.SetIsActive(true)
	_ = group.SetUpdateRate(uint32(dev.poll.Milliseconds()))
	items := group.OPCItems()
	for i := range dev.tags {
		item, err := items.AddItem(dev.tags[i].Config.ItemID)
		if err != nil {
			item = nil
			group.Release()
			_ = groups.Release()
			_ = server.Disconnect()
			return fmt.Errorf("add item %q: %w", dev.tags[i].Config.ItemID, err)
		}
		if err := item.SetIsActive(true); err != nil {
			item.Release()
			group.Release()
			_ = groups.Release()
			_ = server.Disconnect()
			return fmt.Errorf("activate item %q: %w", dev.tags[i].Config.ItemID, err)
		}
		dev.tags[i].Item = item
		dev.index[dev.tags[i].Config.Name] = dev.tags[i]
	}
	dev.server, dev.group = server, group
	log.Printf("opcda initialized: device=%s server=%s vendor=%s group=%s items=%d", dev.cfg.Name, server.GetServerName(), valueOr(server.GetVendorInfo()), dev.cfg.GroupName, len(dev.tags))
	return nil
}

func valueOr(v string, err error) string {
	if err != nil {
		return "<unknown>"
	}
	return v
}

func (dev *Device) close() {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.group != nil {
		dev.group.Release()
		dev.group = nil
	}
	if dev.server != nil {
		_ = dev.server.Disconnect()
		dev.server = nil
	}
	for i := range dev.tags {
		dev.tags[i].Item = nil
		dev.index[dev.tags[i].Config.Name] = dev.tags[i]
	}
}

func (dev *Device) pollOnce(ctx context.Context) error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.server == nil || dev.group == nil {
		return fmt.Errorf("OPC DA device is disconnected")
	}
	for _, tag := range dev.tags {
		if err := ctx.Err(); err != nil {
			return err
		}
		value, quality, ts, err := tag.Item.Read(opcda.OPC_DS_DEVICE)
		if err != nil {
			dev.io.Set(tag.Config.Name, nil, io.Bad, err.Error())
			return err
		}
		if !goodQuality(quality) {
			dev.io.Set(tag.Config.Name, normalize(value, tag.Config), io.Bad, fmt.Sprintf("OPC DA quality=0x%04X", quality))
			continue
		}
		if ts.IsZero() {
			ts = time.Now()
		}
		dev.io.Set(tag.Config.Name, normalize(value, tag.Config), io.Good, "")
		_ = ts
	}
	dev.status.setLastPoll(dev.cfg.Name)
	return nil
}

func goodQuality(q uint16) bool { return q&0x00C0 == 0x00C0 }
func normalize(v any, cfg model.OPCDATagConfig) any {
	var n any
	switch cfg.DataType {
	case model.Bool:
		if x, ok := v.(bool); ok {
			n = x
		} else {
			return v
		}
	case model.Int16:
		n = int16From(v)
	case model.UInt16:
		n = uint16From(v)
	case model.Int32:
		n = int32From(v)
	case model.UInt32:
		n = uint32From(v)
	case model.Float32:
		n = float32From(v)
	case model.Float64:
		n = float64From(v)
	default:
		n = v
	}
	if f, ok := n.(float64); ok {
		return f*cfg.Scale + cfg.Offset
	}
	if f, ok := n.(float32); ok {
		return f*float32(cfg.Scale) + float32(cfg.Offset)
	}
	return n
}
func num(v any) float64 {
	switch x := v.(type) {
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case float32:
		return float64(x)
	case float64:
		return x
	}
	return 0
}
func int16From(v any) int16     { return int16(num(v)) }
func uint16From(v any) uint16   { return uint16(num(v)) }
func int32From(v any) int32     { return int32(num(v)) }
func uint32From(v any) uint32   { return uint32(num(v)) }
func float32From(v any) float32 { return float32(num(v)) }
func float64From(v any) float64 { return num(v) }

func (dev *Device) write(ctx context.Context, cfg model.OPCDATagConfig, value any) error {
	if !cfg.Writable {
		return fmt.Errorf("tag %s is read-only", cfg.Name)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.server == nil {
		return fmt.Errorf("OPC DA device %s is disconnected", dev.cfg.Name)
	}
	tag := dev.index[cfg.Name]
	return tag.Item.Write(value)
}
func (dev *Device) setConnected() {
	dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) { st.Connected = true; st.LastConnected = time.Now(); st.Error = "" })
}
func (dev *Device) setBad(err error) {
	dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) { st.Connected = false; st.Error = err.Error() })
	dev.ioBadAll(err)
}
func (dev *Device) ioBadAll(err error) {
	for _, t := range dev.tags {
		dev.io.Set(t.Config.Name, nil, io.Bad, err.Error())
	}
}

func (s *statusStore) setLastPoll(name string) {
	s.set(name, func(st *driver.DeviceStatus) { st.LastPoll = time.Now() })
}
