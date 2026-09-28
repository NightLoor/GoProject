package config

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"modbus-tcp-driver-v3/internal/model"
)

// LoadAlarmCSV loads alarm rules from configs/Alarm.csv and validates them
// against the currently loaded Tags.csv / IO definitions.
// Columns: tag,data_type,enabled,alarm_value,low,high,unit,message
func LoadAlarmCSV(path string, cfg model.Config) (model.AlarmConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultAlarmConfig(cfg), nil
		}
		return model.AlarmConfig{}, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return model.AlarmConfig{}, fmt.Errorf("read Alarm.csv header: %w", err)
	}
	columns := make(map[string]int, len(header))
	for i, h := range header {
		columns[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"tag", "enabled", "alarm_value", "low", "high", "unit", "message"} {
		if _, ok := columns[required]; !ok {
			return model.AlarmConfig{}, fmt.Errorf("Alarm.csv missing required column %q", required)
		}
	}

	types := AlarmTagTypes(cfg)
	descriptions := AlarmTagDescriptions(cfg)
	seen := make(map[string]struct{})
	rules := make([]model.AlarmRule, 0, len(types))
	line := 1
	for {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		line++
		if readErr != nil {
			return model.AlarmConfig{}, fmt.Errorf("read Alarm.csv line %d: %w", line, readErr)
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
		tag := get("tag")
		if tag == "" {
			return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: tag is empty", line)
		}
		actualType, ok := types[tag]
		if !ok {
			// A row may belong to a currently disabled protocol. Keep the row in
			// Alarm.csv so it can become active again when that driver is enabled,
			// but do not expose it as an active IOManager alarm rule.
			continue
		}
		if _, exists := seen[tag]; exists {
			return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: duplicate tag %q", line, tag)
		}
		seen[tag] = struct{}{}

		enabled, err := strconv.ParseBool(defaultString(get("enabled"), "false"))
		if err != nil {
			return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: invalid enabled %q", line, get("enabled"))
		}
		rule := model.AlarmRule{
			Tag:      tag,
			Enabled:  enabled,
			DataType: actualType,
			Unit:     get("unit"),
			Message:  get("message"),
		}
		if rule.Message == "" {
			rule.Message = descriptions[tag]
		}

		configuredType := strings.ToLower(get("data_type"))
		if configuredType != "" && configuredType != string(actualType) {
			return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: data_type %q does not match IO type %q for tag %q", line, configuredType, actualType, tag)
		}

		if actualType == model.Bool {
			alarmValue := get("alarm_value")
			if alarmValue != "" {
				v, parseErr := strconv.ParseBool(alarmValue)
				if parseErr != nil {
					return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: invalid alarm_value %q for bool tag", line, alarmValue)
				}
				rule.AlarmValue = &v
			} else if enabled {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: bool tag %q requires alarm_value=true/false when enabled", line, tag)
			}
			if get("low") != "" || get("high") != "" {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: bool tag %q cannot use low/high", line, tag)
			}
		} else {
			if get("alarm_value") != "" {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: numeric tag %q cannot use alarm_value", line, tag)
			}
			low, err := parseOptionalFloat(get("low"))
			if err != nil || (low != nil && (math.IsNaN(*low) || math.IsInf(*low, 0))) {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: invalid low %q", line, get("low"))
			}
			high, err := parseOptionalFloat(get("high"))
			if err != nil || (high != nil && (math.IsNaN(*high) || math.IsInf(*high, 0))) {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: invalid high %q", line, get("high"))
			}
			rule.Low, rule.High = low, high
			if rule.Low != nil && rule.High != nil && *rule.Low > *rule.High {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: low cannot be greater than high", line)
			}
			if enabled && rule.Low == nil && rule.High == nil {
				return model.AlarmConfig{}, fmt.Errorf("Alarm.csv line %d: numeric tag %q requires low or high when enabled", line, tag)
			}
		}
		rules = append(rules, rule)
	}

	// Automatically create disabled rows for any new IOManager variable.
	for tag, dataType := range types {
		if _, exists := seen[tag]; exists {
			continue
		}
		rule := model.AlarmRule{Tag: tag, Enabled: false, DataType: dataType, Message: descriptions[tag]}
		if dataType == model.Bool {
			v := false
			rule.AlarmValue = &v
		}
		rules = append(rules, rule)
	}

	sortAlarmRules(rules)
	return model.AlarmConfig{Enabled: true, Alarms: rules}, nil
}

func defaultAlarmConfig(cfg model.Config) model.AlarmConfig {
	types := AlarmTagTypes(cfg)
	descriptions := AlarmTagDescriptions(cfg)
	rules := make([]model.AlarmRule, 0, len(types))
	for tag, dataType := range types {
		rule := model.AlarmRule{Tag: tag, DataType: dataType, Message: descriptions[tag]}
		if dataType == model.Bool {
			v := false
			rule.AlarmValue = &v
		}
		rules = append(rules, rule)
	}
	sortAlarmRules(rules)
	return model.AlarmConfig{Enabled: true, Alarms: rules}
}

func SaveAlarmCSV(path string, cfg model.AlarmConfig) error {
	if err := validateAlarmRules(cfg.Alarms); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(dir, ".Alarm-*.csv")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	w := csv.NewWriter(tmp)
	if err := w.Write([]string{"tag", "data_type", "enabled", "alarm_value", "low", "high", "unit", "message"}); err != nil {
		_ = tmp.Close()
		return err
	}
	rules := append([]model.AlarmRule(nil), cfg.Alarms...)
	sortAlarmRules(rules)
	for _, rule := range rules {
		alarmValue := ""
		if rule.AlarmValue != nil {
			alarmValue = strconv.FormatBool(*rule.AlarmValue)
		}
		low := ""
		if rule.Low != nil {
			low = strconv.FormatFloat(*rule.Low, 'f', -1, 64)
		}
		high := ""
		if rule.High != nil {
			high = strconv.FormatFloat(*rule.High, 'f', -1, 64)
		}
		if err := w.Write([]string{
			rule.Tag,
			string(rule.DataType),
			strconv.FormatBool(rule.Enabled),
			alarmValue,
			low,
			high,
			rule.Unit,
			rule.Message,
		}); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// ValidateAlarmConfig validates an in-memory update. The authoritative type is the
// IO definition; the request's DataType must match it.
func ValidateAlarmConfig(cfg model.Config, alarm model.AlarmConfig) error {
	types := AlarmTagTypes(cfg)
	seen := make(map[string]struct{}, len(alarm.Alarms))
	for _, rule := range alarm.Alarms {
		dataType, ok := types[rule.Tag]
		if !ok {
			return fmt.Errorf("alarm tag %q is not found in IO definitions", rule.Tag)
		}
		if _, ok := seen[rule.Tag]; ok {
			return fmt.Errorf("duplicate alarm rule for tag %q", rule.Tag)
		}
		seen[rule.Tag] = struct{}{}
		if rule.DataType != dataType {
			return fmt.Errorf("alarm tag %q data_type %q does not match IO type %q", rule.Tag, rule.DataType, dataType)
		}
		if dataType == model.Bool {
			if rule.Low != nil || rule.High != nil {
				return fmt.Errorf("bool tag %q cannot use low/high", rule.Tag)
			}
			if rule.Enabled && rule.AlarmValue == nil {
				return fmt.Errorf("bool tag %q requires alarm_value when enabled", rule.Tag)
			}
		} else {
			if rule.AlarmValue != nil {
				return fmt.Errorf("numeric tag %q cannot use alarm_value", rule.Tag)
			}
			if rule.Low != nil && rule.High != nil && *rule.Low > *rule.High {
				return fmt.Errorf("alarm tag %q low cannot be greater than high", rule.Tag)
			}
			if rule.Enabled && rule.Low == nil && rule.High == nil {
				return fmt.Errorf("numeric tag %q requires low or high when enabled", rule.Tag)
			}
		}
	}
	return nil
}

func AlarmTagTypes(cfg model.Config) map[string]model.DataType {
	result := make(map[string]model.DataType)
	for _, d := range cfg.Devices {
		for _, t := range d.Tags {
			result[t.Name] = t.DataType
		}
	}
	for _, d := range cfg.OPCDADevices {
		for _, t := range d.Tags {
			result[t.Name] = t.DataType
		}
	}
	return result
}

func AlarmTagDescriptions(cfg model.Config) map[string]string {
	result := make(map[string]string)
	for _, d := range cfg.Devices {
		for _, t := range d.Tags {
			result[t.Name] = t.Description
		}
	}
	for _, d := range cfg.OPCDADevices {
		for _, t := range d.Tags {
			result[t.Name] = t.Description
		}
	}
	return result
}

func validateAlarmRules(rules []model.AlarmRule) error {
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		if strings.TrimSpace(rule.Tag) == "" {
			return fmt.Errorf("alarm tag is empty")
		}
		if _, ok := seen[rule.Tag]; ok {
			return fmt.Errorf("duplicate alarm rule for tag %q", rule.Tag)
		}
		seen[rule.Tag] = struct{}{}
		if rule.DataType == model.Bool {
			if rule.Low != nil || rule.High != nil {
				return fmt.Errorf("bool tag %q cannot use low/high", rule.Tag)
			}
			if rule.Enabled && rule.AlarmValue == nil {
				return fmt.Errorf("bool tag %q requires alarm_value", rule.Tag)
			}
		} else {
			if rule.AlarmValue != nil {
				return fmt.Errorf("numeric tag %q cannot use alarm_value", rule.Tag)
			}
			if rule.Low != nil && rule.High != nil && *rule.Low > *rule.High {
				return fmt.Errorf("alarm tag %q low cannot be greater than high", rule.Tag)
			}
		}
	}
	return nil
}

func parseOptionalFloat(v string) (*float64, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return &n, err
}

func sortAlarmRules(rules []model.AlarmRule) {
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Tag < rules[j].Tag })
}
