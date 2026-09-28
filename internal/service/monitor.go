package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"modbus-tcp-driver-v3/internal/config"
	"modbus-tcp-driver-v3/internal/driver"
	"modbus-tcp-driver-v3/internal/io"
	"modbus-tcp-driver-v3/internal/model"
)

type DriverPort = driver.Port

type DeviceStatus struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Connected     bool   `json:"connected"`
	LastConnected string `json:"last_connected,omitempty"`
	LastPoll      string `json:"last_poll,omitempty"`
	Protocol      string `json:"protocol"`
	Error         string `json:"error,omitempty"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

type MonitorService struct {
	mu         sync.RWMutex
	cfg        model.Config
	points     *io.Manager
	driver     DriverPort
	alarmSaver func(model.AlarmConfig) error
}

func NewMonitorService(cfg model.Config, points *io.Manager, driver DriverPort) *MonitorService {
	return &MonitorService{cfg: cfg, points: points, driver: driver}
}

func (s *MonitorService) SetAlarmSaver(saver func(model.AlarmConfig) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alarmSaver = saver
}

func (s *MonitorService) Config() model.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *MonitorService) Points() []io.Point {
	return s.points.All()
}

func (s *MonitorService) Point(name string) (io.Point, bool) {
	return s.points.Get(name)
}

func (s *MonitorService) HistoryRange(name string, start, end time.Time) ([]io.Sample, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("tag is required")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("history start time must be before end time")
	}
	if end.Sub(start) > 10*24*time.Hour {
		return nil, fmt.Errorf("history time span cannot exceed 10 days")
	}
	return s.points.HistoryRange(name, start, end)
}

func (s *MonitorService) History(name string, limit int) ([]io.Sample, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("tag is required")
	}
	if limit <= 0 {
		limit = 60
	}
	if limit > 5000 {
		limit = 5000
	}
	return s.points.History(name, limit)
}

func (s *MonitorService) Alarms() []io.AlarmStatus {
	return s.points.Alarms()
}

func (s *MonitorService) AlarmConfig() model.AlarmConfig {
	return s.points.AlarmConfig()
}

func (s *MonitorService) UpdateAlarmConfig(incoming model.AlarmConfig) error {
	s.mu.RLock()
	currentCfg := s.cfg
	saver := s.alarmSaver
	currentAlarm := s.points.AlarmConfig()
	s.mu.RUnlock()

	merged := currentAlarm
	if merged.Alarms == nil {
		merged = incoming
	} else {
		rules := make(map[string]model.AlarmRule, len(merged.Alarms))
		for _, rule := range merged.Alarms {
			rules[rule.Tag] = rule
		}
		for _, rule := range incoming.Alarms {
			rules[rule.Tag] = rule
		}
		merged.Alarms = merged.Alarms[:0]
		for _, rule := range rules {
			merged.Alarms = append(merged.Alarms, rule)
		}
		merged.Enabled = true
	}

	// Refresh authoritative DataType from the current IO definitions and make sure
	// any newly discovered IO variable gets a disabled default rule.
	types := config.AlarmTagTypes(currentCfg)
	for tag, dataType := range types {
		found := false
		for i := range merged.Alarms {
			if merged.Alarms[i].Tag != tag {
				continue
			}
			merged.Alarms[i].DataType = dataType
			found = true
		}
		if !found {
			rule := model.AlarmRule{Tag: tag, DataType: dataType, Enabled: false}
			if dataType == model.Bool {
				def := false
				rule.AlarmValue = &def
			}
			merged.Alarms = append(merged.Alarms, rule)
		}
	}
	if err := config.ValidateAlarmConfig(currentCfg, merged); err != nil {
		return err
	}
	if saver == nil {
		return fmt.Errorf("alarm configuration saver is not configured")
	}
	if err := saver(merged); err != nil {
		return err
	}

	s.points.SetAlarmConfig(merged)
	s.mu.Lock()
	s.cfg.Alarm = merged
	s.mu.Unlock()
	return nil
}

func (s *MonitorService) AlarmHistory(tag string, start, end time.Time) ([]io.Event, error) {
	if !start.Before(end) {
		return nil, fmt.Errorf("alarm history start time must be before end time")
	}
	return s.points.AlarmEvents(strings.TrimSpace(tag), start, end)
}

func (s *MonitorService) Events(limit int) []io.Event {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.points.Events(limit)
}

func (s *MonitorService) Devices() []DeviceStatus {
	statuses := s.driver.DeviceStatuses()
	result := make([]DeviceStatus, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, DeviceStatus{
			Name:          status.Name,
			Protocol:      status.Protocol,
			Address:       status.Address,
			Connected:     status.Connected,
			LastConnected: formatTime(status.LastConnected),
			LastPoll:      formatTime(status.LastPoll),
			Error:         status.Error,
		})
	}
	return result
}

func (s *MonitorService) Write(ctx context.Context, tagName string, value any) error {
	if strings.TrimSpace(tagName) == "" {
		return fmt.Errorf("tag is required")
	}
	return s.driver.Write(ctx, tagName, value)
}
