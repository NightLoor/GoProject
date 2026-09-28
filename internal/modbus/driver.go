package modbus

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"modbus-tcp-driver-v3/internal/driver"
	"modbus-tcp-driver-v3/internal/io"
	"modbus-tcp-driver-v3/internal/model"

	gm "github.com/goburrow/modbus"
)

type Device struct {
	cfg       model.DeviceConfig
	client    *Client
	io        *io.Manager
	status    *statusStore
	tags      []Tag
	index     map[string]Tag
	reconnect time.Duration
	poll      time.Duration
}

type Driver struct {
	devices []*Device
	poll    time.Duration
	status  *statusStore
}

// Compile-time interface check: Modbus Driver must satisfy the common driver.Port contract.
var _ driver.Port = (*Driver)(nil)

func NewDriver(cfg model.Config, ioManager *io.Manager) (*Driver, error) {
	if !cfg.IsModbusEnabled() {
		return &Driver{status: newStatusStore()}, nil
	}
	poll := time.Duration(cfg.PollIntervalMS) * time.Millisecond
	if poll <= 0 {
		poll = time.Second
	}

	d := &Driver{poll: poll, status: newStatusStore()}

	for _, dc := range cfg.Devices {
		dev := &Device{
			cfg:       dc,
			client:    NewClient(dc.Address, durationOr(dc.TimeoutMS, 2000)),
			io:        ioManager,
			status:    d.status,
			index:     make(map[string]Tag),
			reconnect: durationOr(dc.ReconnectIntervalMS, 3000),
			poll:      poll,
		}

		for _, tc := range dc.Tags {
			if tc.Name == "" {
				return nil, fmt.Errorf("device %s contains empty tag name", dc.Name)
			}
			info, err := ParseAddress(tc.Address, tc.RegisterType)
			if err != nil {
				return nil, fmt.Errorf("device=%s tag=%s: %w", dc.Name, tc.Name, err)
			}
			if tc.Scale == 0 {
				tc.Scale = 1
			}
			if tc.ByteOrder == "" {
				tc.ByteOrder = model.ABCD
			}
			width := uint16(1)
			if info.RegisterType == model.Coil || info.RegisterType == model.DiscreteInput {
				width = 1
			} else {
				width = RegisterCount(tc.DataType)
			}
			t := Tag{Config: tc, Offset: info.Offset, RegisterType: info.RegisterType, Width: width}
			if _, exists := dev.index[tc.Name]; exists {
				return nil, fmt.Errorf("duplicate tag %s on device %s", tc.Name, dc.Name)
			}
			dev.tags = append(dev.tags, t)
			dev.index[tc.Name] = t
		}

		d.devices = append(d.devices, dev)
		d.status.set(dc.Name, func(st *driver.DeviceStatus) {
			st.Name = dc.Name
			st.Protocol = "Modbus TCP"
			st.Address = dc.Address
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
		go func(x *Device) {
			defer wg.Done()
			x.run(ctx)
		}(dev)
	}
	wg.Wait()
}

func (d *Driver) Write(ctx context.Context, tagName string, value any) error {
	for _, dev := range d.devices {
		if tag, ok := dev.index[tagName]; ok {
			return dev.write(ctx, tag, value)
		}
	}
	return fmt.Errorf("tag not found: %s", tagName)
}

func (d *Driver) DeviceStatuses() []driver.DeviceStatus {
	return d.status.all()
}

func (d *Driver) HasTag(tagName string) bool {
	for _, dev := range d.devices {
		if _, ok := dev.index[tagName]; ok {
			return true
		}
	}
	return false
}

func (dev *Device) run(ctx context.Context) {
	for {
		//尝试连接
		if err := dev.client.Connect(); err != nil {
			dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) {
				st.Connected = false
				st.Error = err.Error()
			})
			dev.markAllBad(err)
			if !waitOrCancel(ctx, dev.reconnect) {
				return
			}
			continue
		}

		dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) {
			st.Connected = true
			st.LastConnected = time.Now()
			st.Error = ""
		})
		log.Printf("modbus device connected: %s (%s)", dev.cfg.Name, dev.cfg.Address)

		//第一次轮询
		if err := dev.pollOnce(ctx); err != nil {
			log.Printf("modbus device poll error: %s: %v", dev.cfg.Name, err)
			dev.client.Close()
			dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) {
				st.Connected = false
				st.Error = err.Error()
			})
			dev.markAllBad(err)
			if !waitOrCancel(ctx, dev.reconnect) {
				return
			}
			continue
		}
		//轮询定时器循环
		ticker := time.NewTicker(dev.poll)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				dev.client.Close()
				return
			case <-ticker.C:
				if err := dev.pollOnce(ctx); err != nil {
					log.Printf("modbus device poll error: %s: %v", dev.cfg.Name, err)
					ticker.Stop()
					dev.client.Close()
					dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) {
						st.Connected = false
						st.Error = err.Error()
					})
					dev.markAllBad(err)
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

// waitOrCancel 等待d时间，ctx取消返回false
func waitOrCancel(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// pollOnce 执行一轮完整采集
func (dev *Device) pollOnce(ctx context.Context) error {
	groups := make(map[string][]Tag)
	for _, t := range dev.tags {
		key := fmt.Sprintf("%d/%s", t.Config.SlaveID, t.RegisterType)
		groups[key] = append(groups[key], t)
	}

	for _, tags := range groups {
		var maxQuantity uint16
		if tags[0].RegisterType == model.Coil || tags[0].RegisterType == model.DiscreteInput {
			maxQuantity = dev.cfg.BitBatchSize
		} else {
			maxQuantity = dev.cfg.RegisterBatchSize
		}
		batches, err := BuildBatches(tags, dev.cfg.BatchGap, maxQuantity)
		if err != nil {
			return err
		}
		for _, batch := range batches {
			if err := dev.readBatch(ctx, batch); err != nil {
				log.Printf("modbus read batch failed dev=%s slave=%d start=%d err=%v",
					dev.cfg.Name, batch.SlaveID, batch.Start, err)
				return err
			}
		}
	}
	dev.status.set(dev.cfg.Name, func(st *driver.DeviceStatus) {
		st.LastPoll = time.Now()
	})
	return nil
}

// readBatch 执行一次modbus批量读请求，解析返回数据写入ioManager
func (dev *Device) readBatch(ctx context.Context, batch Batch) error {
	return dev.client.WithCtx(ctx, func(c gm.Client, h *gm.TCPClientHandler) error {
		h.SlaveId = batch.SlaveID

		var raw []byte
		var err error

		switch batch.RegisterType {
		case model.Coil:
			raw, err = c.ReadCoils(batch.Start, batch.Quantity)
		case model.DiscreteInput:
			raw, err = c.ReadDiscreteInputs(batch.Start, batch.Quantity)
		case model.HoldingRegister:
			raw, err = c.ReadHoldingRegisters(batch.Start, batch.Quantity)
		case model.InputRegister:
			raw, err = c.ReadInputRegisters(batch.Start, batch.Quantity)
		default:
			return fmt.Errorf("unsupported register type %s", batch.RegisterType)
		}
		if err != nil {
			//请求失败，本组所有tag置Bad
			for _, tag := range batch.Tags {
				dev.io.Set(tag.Config.Name, nil, io.Bad, err.Error())
			}
			// 判断：是否为连接断开类错误；goburrow库连接断开错误包含 "EOF","connection refused","broken pipe"
			errStr := err.Error()
			if strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "broken pipe") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "modbus client is not connected") {
				return err // 网络连接问题：向上返回，触发重连
			}
			// modbus协议异常（如非法地址）：只标记点位，返回nil，不断开TCP
			log.Printf("modbus slave exception batch slave=%d start=%d err=%v", batch.SlaveID, batch.Start, err)
			return nil
		}
		//解析每个tag
		for _, tag := range batch.Tags {
			value, decodeErr := extractTagValue(raw, batch, tag)
			if decodeErr != nil {
				dev.io.Set(tag.Config.Name, nil, io.Bad, decodeErr.Error())
				continue
			}
			//应用scale offset计算工程值
			value = Scale(value, tag.Config.Scale, tag.Config.Offset)
			dev.io.Set(tag.Config.Name, value, io.Good, "")
		}
		return nil
	})
}

// extractTagValue 从batch返回原始字节中提取单个tag的值
func extractTagValue(raw []byte, batch Batch, tag Tag) (any, error) {
	if tag.RegisterType == model.Coil || tag.RegisterType == model.DiscreteInput {
		bitIndex := int(tag.Offset - batch.Start)
		byteIndex := bitIndex / 8
		bit := uint(bitIndex % 8)
		if byteIndex >= len(raw) {
			return nil, fmt.Errorf("coil response too short for %s", tag.Config.Name)
		}
		return (raw[byteIndex] & (1 << bit)) != 0, nil
	}
	//寄存器类型
	byteOffset := int(tag.Offset-batch.Start) * 2
	byteLen := int(tag.Width) * 2
	if byteOffset+byteLen > len(raw) {
		return nil, fmt.Errorf("register response too short for %s", tag.Config.Name)
	}
	return Decode(raw[byteOffset:byteOffset+byteLen], tag.Config.DataType, tag.Config.ByteOrder)
}

// write 写单个tag
func (dev *Device) write(ctx context.Context, tag Tag, value any) error {
	// 第一步：配置校验，如果tag不允许写直接返回错误
	if !tag.Config.Writable {
		return fmt.Errorf("tag %s is not writable", tag.Config.Name)
	}
	return dev.client.WithCtx(ctx, func(c gm.Client, h *gm.TCPClientHandler) error {

		h.SlaveId = tag.Config.SlaveID

		if tag.RegisterType == model.Coil {

			b, ok := value.(bool)
			if !ok {
				return fmt.Errorf(
					"coil tag %s requires bool value",
					tag.Config.Name,
				)
			}
			// Modbus协议：WriteSingleCoil，线圈开启=0xFF00，线圈关闭=0x0000，不能传true/false，必须传这两个固定uint16值
			var coilValue uint16
			if b {
				coilValue = 0xFF00
			} else {
				coilValue = 0x0000
			}

			_, err := c.WriteSingleCoil(
				tag.Offset,
				coilValue,
			)

			return err
		}

		if tag.RegisterType != model.HoldingRegister {
			return fmt.Errorf("tag %s register type %s is not writable", tag.Config.Name, tag.RegisterType)
		}

		rawValue, err := Unscale(value, tag.Config.Scale, tag.Config.Offset)
		if err != nil {
			return err
		}

		raw, err := Encode(rawValue, tag.Config.DataType, tag.Config.ByteOrder)
		if err != nil {
			return err
		}

		if len(raw) == 2 {
			reg := uint16(raw[0])<<8 | uint16(raw[1])
			_, err = c.WriteSingleRegister(tag.Offset, reg)
			return err
		}

		_, err = c.WriteMultipleRegisters(tag.Offset, uint16(len(raw)/2), raw)
		return err
	})
}

// markAllBad 把本设备所有点位置坏质量
func (dev *Device) markAllBad(err error) {
	for _, tag := range dev.tags {
		dev.io.Set(tag.Config.Name, nil, io.Bad, err.Error())
	}
}
