package config

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"modbus-tcp-driver-v3/internal/model"
)

// LoadJSON reads one protocol/device JSON configuration file and fills defaults.
// Tags are intentionally allowed to be empty because the actual tag definitions
// are loaded from configs/Tags.csv by LoadTagsCSV.
func LoadJSON(path string) (model.Config, error) {
	var cfg model.Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "opcda.json":
		var raw struct {
			Enabled       *bool                     `json:"enabled"`
			Devices       []model.OPCDADeviceConfig `json:"devices"`
			LegacyEnabled *bool                     `json:"opcda_enabled"`
			LegacyDevices []model.OPCDADeviceConfig `json:"opcda_devices"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return cfg, err
		}
		cfg.OPCDADevices = raw.Devices
		if len(cfg.OPCDADevices) == 0 && len(raw.LegacyDevices) > 0 {
			cfg.OPCDADevices = raw.LegacyDevices
		}
		if raw.Enabled != nil {
			cfg.OPCDAEnabled = *raw.Enabled
			cfg.OPCDAEnabledSet = true
		} else if raw.LegacyEnabled != nil {
			cfg.OPCDAEnabled = *raw.LegacyEnabled
			cfg.OPCDAEnabledSet = true
		} else {
			cfg.OPCDAEnabled = true
		}
	default:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
		var raw struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return cfg, err
		}
		if raw.Enabled != nil {
			cfg.ModbusEnabled = *raw.Enabled
			cfg.ModbusEnabledSet = true
		} else {
			cfg.ModbusEnabled = true
		}
	}

	ApplyDefaults(&cfg)
	return cfg, nil
}

func ApplyDefaults(cfg *model.Config) {
	if cfg.PollIntervalMS <= 0 {
		cfg.PollIntervalMS = 1000
	}
	for i := range cfg.Devices {
		if cfg.Devices[i].TimeoutMS <= 0 {
			cfg.Devices[i].TimeoutMS = 2000
		}
		if cfg.Devices[i].ReconnectIntervalMS <= 0 {
			cfg.Devices[i].ReconnectIntervalMS = 3000
		}
		if cfg.Devices[i].RegisterBatchSize == 0 || cfg.Devices[i].RegisterBatchSize > 125 {
			cfg.Devices[i].RegisterBatchSize = 125
		}
		if cfg.Devices[i].BitBatchSize == 0 || cfg.Devices[i].BitBatchSize > 2000 {
			cfg.Devices[i].BitBatchSize = 2000
		}
	}
	for i := range cfg.OPCDADevices {
		if cfg.OPCDADevices[i].TimeoutMS <= 0 {
			cfg.OPCDADevices[i].TimeoutMS = 10000
		}
		if cfg.OPCDADevices[i].ReconnectIntervalMS <= 0 {
			cfg.OPCDADevices[i].ReconnectIntervalMS = 5000
		}
		if cfg.OPCDADevices[i].GroupName == "" {
			cfg.OPCDADevices[i].GroupName = "GoMonitor"
		}
	}
}

// LoadTagsCSV loads all protocol tags from one unified CSV file.
// Required columns:
// protocol,device,tag_name,address,data_type,writable,slave_id,scale,offset,byte_order,description
func LoadTagsCSV(path string, cfg *model.Config) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read Tags.csv header: %w", err)
	}
	columns := make(map[string]int, len(header))
	for i, h := range header {
		columns[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"protocol", "device", "tag_name", "address", "data_type", "writable"} {
		if _, ok := columns[required]; !ok {
			return fmt.Errorf("Tags.csv missing required column %q", required)
		}
	}

	// Clear JSON tag arrays so CSV is the single source of truth.
	for i := range cfg.Devices {
		cfg.Devices[i].Tags = nil
	}
	for i := range cfg.OPCDADevices {
		cfg.OPCDADevices[i].Tags = nil
	}

	modbusDevices := make(map[string]*model.DeviceConfig, len(cfg.Devices))
	for i := range cfg.Devices {
		modbusDevices[cfg.Devices[i].Name] = &cfg.Devices[i]
	}
	opcdaDevices := make(map[string]*model.OPCDADeviceConfig, len(cfg.OPCDADevices))
	for i := range cfg.OPCDADevices {
		opcdaDevices[cfg.OPCDADevices[i].Name] = &cfg.OPCDADevices[i]
	}
	seen := make(map[string]string)
	line := 1
	for {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		line++
		if readErr != nil {
			return fmt.Errorf("read Tags.csv line %d: %w", line, readErr)
		}
		if len(record) == 0 || allBlank(record) {
			continue
		}
		get := func(name string) string {
			idx, ok := columns[name]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}
		protocol := strings.ToUpper(get("protocol"))
		device := get("device")
		name := get("tag_name")
		address := get("address")
		dataType := model.DataType(strings.ToLower(get("data_type")))
		description := get("description")
		if device == "" || name == "" || address == "" || dataType == "" {
			return fmt.Errorf("Tags.csv line %d: protocol/device/tag_name/address/data_type cannot be empty", line)
		}
		key := protocol + ":" + device + ":" + name
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("Tags.csv line %d: duplicate tag %s (previous %s)", line, name, prev)
		}
		seen[key] = fmt.Sprintf("line %d", line)
		scale := parseFloat(get("scale"), 1)
		offset := parseFloat(get("offset"), 0)
		writable, err := strconv.ParseBool(defaultString(get("writable"), "false"))
		if err != nil {
			return fmt.Errorf("Tags.csv line %d: invalid writable %q", line, get("writable"))
		}
		historyEnabled, err := strconv.ParseBool(defaultString(get("history"), "false"))
		if err != nil {
			return fmt.Errorf("Tags.csv line %d: invalid history %q", line, get("history"))
		}

		switch protocol {
		case "MODBUS":
			if !cfg.IsModbusEnabled() {
				continue
			}
			dc := modbusDevices[device]
			if dc == nil {
				return fmt.Errorf("Tags.csv line %d: Modbus device %q not found in modbus.json", line, device)
			}
			slaveID := byte(parseUint(get("slave_id"), 1))
			registerType := model.RegisterType(strings.ToLower(get("register_type")))
			if registerType == "" {
				return fmt.Errorf("Tags.csv line %d: register_type is required for Modbus", line)
			}
			byteOrder := model.ByteOrder(strings.ToUpper(get("byte_order")))
			if byteOrder == "" {
				byteOrder = model.ABCD
			}
			dc.Tags = append(dc.Tags, model.TagConfig{
				Name: name, Address: address, RegisterType: registerType, DataType: dataType,
				SlaveID: slaveID, Writable: writable, Scale: scale, Offset: offset, ByteOrder: byteOrder, Description: description, History: historyEnabled,
			})
		case "OPCDA", "OPC DA":
			if !cfg.IsOPCDAEnabled() {
				continue
			}
			dc := opcdaDevices[device]
			if dc == nil {
				return fmt.Errorf("Tags.csv line %d: OPC DA device %q not found in opcda.json", line, device)
			}
			dc.Tags = append(dc.Tags, model.OPCDATagConfig{
				Name: name, ItemID: address, DataType: dataType, Writable: writable, Scale: scale, Offset: offset, Description: description, History: historyEnabled,
			})
		default:
			return fmt.Errorf("Tags.csv line %d: unsupported protocol %q", line, protocol)
		}
	}
	ApplyDefaults(cfg)
	return nil
}

func HistoryFlags(cfg model.Config) map[string]bool {
	flags := make(map[string]bool)
	for _, device := range cfg.Devices {
		for _, tag := range device.Tags {
			flags[tag.Name] = tag.History
		}
	}
	for _, device := range cfg.OPCDADevices {
		for _, tag := range device.Tags {
			flags[tag.Name] = tag.History
		}
	}
	return flags
}

func ValidateConfig(cfg model.Config) error {
	seenTags := make(map[string]string)
	if cfg.IsModbusEnabled() {
		for _, d := range cfg.Devices {
			if d.Name == "" {
				return fmt.Errorf("device name is empty")
			}
			if d.Address == "" {
				return fmt.Errorf("device %s address is empty", d.Name)
			}
			if len(d.Tags) == 0 {
				return fmt.Errorf("device %s has no tags", d.Name)
			}
			for _, t := range d.Tags {
				if t.Name == "" {
					return fmt.Errorf("device %s contains empty tag name", d.Name)
				}
				if previous, exists := seenTags[t.Name]; exists {
					return fmt.Errorf("duplicate tag %s: devices %s and %s", t.Name, previous, d.Name)
				}
				seenTags[t.Name] = d.Name
			}
		}
	}
	if cfg.IsOPCDAEnabled() {
		for _, d := range cfg.OPCDADevices {
			if d.Name == "" {
				return fmt.Errorf("opc da device name is empty")
			}
			if d.ProgID == "" {
				return fmt.Errorf("opc da device %s prog_id is empty", d.Name)
			}
			if len(d.Tags) == 0 {
				return fmt.Errorf("opc da device %s has no tags", d.Name)
			}
			for _, t := range d.Tags {
				if t.Name == "" {
					return fmt.Errorf("opc da device %s contains empty tag name", d.Name)
				}
				if t.ItemID == "" {
					return fmt.Errorf("opc da device %s tag %s item_id is empty", d.Name, t.Name)
				}
				if previous, exists := seenTags[t.Name]; exists {
					return fmt.Errorf("duplicate tag %s: devices %s and %s", t.Name, previous, d.Name)
				}
				seenTags[t.Name] = d.Name
			}
		}
	}
	return nil
}

func parseFloat(v string, fallback float64) float64 {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
		return n
	}
	return fallback
}

func parseUint(v string, fallback uint64) uint64 {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	if n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64); err == nil {
		return n
	}
	return fallback
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func allBlank(row []string) bool {
	for _, v := range row {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
