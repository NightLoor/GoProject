package io

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"modbus-tcp-driver-v3/internal/model"
)

type Quality string

const (
	Good Quality = "Good"
	Bad  Quality = "Bad"
)

type Point struct {
	Name        string         `json:"name"`
	Value       any            `json:"value"`
	Quality     Quality        `json:"quality"`
	Timestamp   time.Time      `json:"timestamp"`
	Error       string         `json:"error,omitempty"`
	History     bool           `json:"history"`
	DataType    model.DataType `json:"data_type"`
	Description string         `json:"description,omitempty"`
}

type PointDefinition struct {
	Name        string
	DataType    model.DataType
	Description string
	History     bool
}

type Sample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     any       `json:"value"`
	Quality   Quality   `json:"quality"`
	Error     string    `json:"error,omitempty"`
}

// HistoryStore is intentionally defined in the IO layer so protocol drivers
// do not need to know anything about the database implementation.
type HistoryStore interface {
	Append(tag string, sample Sample) error
	Query(tag string, limit int) ([]Sample, error)
	QueryRange(tag string, start, end time.Time, maxPoints int) ([]Sample, error)
	AppendAlarmEvent(event Event) error
	QueryAlarmEvents(tag string, start, end time.Time) ([]Event, error)
}

type Event struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Tag       string    `json:"tag,omitempty"`
	Message   string    `json:"message"`
	Value     any       `json:"value,omitempty"`
	Limit     *float64  `json:"limit,omitempty"`
	Expected  *bool     `json:"expected,omitempty"`
	Unit      string    `json:"unit,omitempty"`
}

type AlarmStatus struct {
	Tag       string    `json:"tag"`
	Value     any       `json:"value"`
	State     string    `json:"state"` // high / low / bool_true / bool_false
	Limit     *float64  `json:"limit,omitempty"`
	Expected  *bool     `json:"expected,omitempty"`
	Unit      string    `json:"unit,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type Manager struct {
	mu                    sync.RWMutex
	points                map[string]Point
	history               map[string][]Sample
	historyEnabled        map[string]bool
	pointDefs             map[string]PointDefinition
	historyStore          HistoryStore
	historyGlobalEnabled  bool
	historyInterval       time.Duration
	historyLastBucket     map[string]int64
	historyQueryMaxPoints int
	events                []Event
	activeAlarms          map[string]AlarmStatus
	alarmConfig           model.AlarmConfig
	nextEventID           int64
	maxHistory            int
	maxEvents             int
}

func NewManager() *Manager {
	return &Manager{
		points:                make(map[string]Point),
		history:               make(map[string][]Sample),
		historyEnabled:        make(map[string]bool),
		historyGlobalEnabled:  true,
		historyInterval:       time.Minute,
		historyLastBucket:     make(map[string]int64),
		historyQueryMaxPoints: 2000,
		pointDefs:             make(map[string]PointDefinition),
		activeAlarms:          make(map[string]AlarmStatus),
		maxHistory:            300,
		maxEvents:             200,
	}
}

func (m *Manager) SetHistoryStore(store HistoryStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.historyStore = store
}

// RegisterPointDefinitions registers the complete IO point list before drivers start.
// This makes the current IOManager variable list available to the API/UI even before
// the first successful device poll.
func (m *Manager) RegisterPointDefinitions(defs []PointDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, def := range defs {
		m.pointDefs[def.Name] = def
		m.historyEnabled[def.Name] = def.History
		if _, ok := m.points[def.Name]; !ok {
			m.points[def.Name] = Point{
				Name:        def.Name,
				Quality:     Bad,
				Error:       "waiting for first value",
				History:     def.History,
				DataType:    def.DataType,
				Description: def.Description,
			}
		}
	}
}

// SetHistoryConfig replaces the history switches. Only tags explicitly set to
// true are written to the historical database.
func (m *Manager) SetHistoryConfig(flags map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.historyEnabled = make(map[string]bool, len(flags))
	for tag, enabled := range flags {
		m.historyEnabled[tag] = enabled
	}
}

// SetHistoryPolicy configures the global persistence switch, sampling interval,
// and maximum points returned for a date-range history query. Per-variable
// persistence is still controlled by Tags.csv's history column.
func (m *Manager) SetHistoryPolicy(enabled bool, interval time.Duration, maxQueryPoints int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if interval <= 0 {
		interval = time.Minute
	}
	if maxQueryPoints <= 0 {
		maxQueryPoints = 2000
	}
	m.historyGlobalEnabled = enabled
	m.historyInterval = interval
	m.historyQueryMaxPoints = maxQueryPoints
	for name, point := range m.points {
		point.History = enabled && m.historyEnabled[name]
		m.points[name] = point
	}
}

func (m *Manager) Set(name string, value any, quality Quality, errText string) {
	m.setAt(time.Now(), name, value, quality, errText)
}

func (m *Manager) setAt(now time.Time, name string, value any, quality Quality, errText string) {
	sample := Sample{Timestamp: now, Value: value, Quality: quality, Error: errText}

	m.mu.Lock()
	old, exists := m.points[name]
	historyEnabled := m.historyGlobalEnabled && m.historyEnabled[name]
	def := m.pointDefs[name]
	shouldPersist := false
	if historyEnabled {
		bucket := now.Unix() / int64(m.historyInterval/time.Second)
		if m.historyLastBucket[name] != bucket {
			m.historyLastBucket[name] = bucket
			shouldPersist = true
		}
	}
	m.points[name] = Point{
		Name:        name,
		Value:       value,
		Quality:     quality,
		Timestamp:   now,
		Error:       errText,
		History:     historyEnabled,
		DataType:    def.DataType,
		Description: def.Description,
	}

	if historyEnabled && shouldPersist {
		samples := append(m.history[name], sample)
		if len(samples) > m.maxHistory {
			samples = samples[len(samples)-m.maxHistory:]
		}
		m.history[name] = samples
	}

	if !exists || old.Quality != quality || old.Error != errText {
		if quality == Bad {
			m.appendEventLocked("quality_bad", name, errText, value, nil, nil, "")
		} else if exists && old.Quality == Bad && quality == Good {
			m.appendEventLocked("quality_recovered", name, "point quality recovered", value, nil, nil, "")
		}
	}
	if quality == Good {
		m.evaluateAlarmLocked(name, value, now)
	}
	store := m.historyStore
	m.mu.Unlock()

	if shouldPersist && store != nil {
		if err := store.Append(name, sample); err != nil {
			logHistoryError(name, err)
		}
	}
}

func (m *Manager) SetAlarmConfig(cfg model.AlarmConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alarmConfig = cfg
	// Re-evaluate existing good values immediately after a configuration change.
	for name, point := range m.points {
		if point.Quality == Good {
			m.evaluateAlarmLocked(name, point.Value, point.Timestamp)
		}
	}
}

func (m *Manager) Alarms() []AlarmStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]AlarmStatus, 0, len(m.activeAlarms))
	for _, alarm := range m.activeAlarms {
		result = append(result, alarm)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Tag < result[j].Tag })
	return result
}

func (m *Manager) AlarmConfig() model.AlarmConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alarmConfig
}

func (m *Manager) evaluateAlarmLocked(name string, value any, now time.Time) {
	active, activeExists := m.activeAlarms[name]
	if !m.alarmConfig.Enabled {
		if activeExists {
			delete(m.activeAlarms, name)
			m.appendEventLocked("alarm_recovered", name, "变量报警功能已停用", value, active.Limit, active.Expected, active.Unit)
		}
		return
	}
	rule, ok := m.findAlarmRuleLocked(name)
	if !ok || !rule.Enabled {
		if activeExists {
			delete(m.activeAlarms, name)
			m.appendEventLocked("alarm_recovered", name, "变量报警已停用，报警恢复", value, active.Limit, active.Expected, active.Unit)
		}
		return
	}

	message := rule.Message
	state := ""
	var limit *float64
	var expected *bool

	if rule.DataType == model.Bool {
		actual, ok := boolValue(value)
		if !ok || rule.AlarmValue == nil {
			return
		}
		expectedValue := *rule.AlarmValue
		expected = &expectedValue
		if actual == expectedValue {
			if message == "" {
				message = fmt.Sprintf("变量为 %t，触发报警", expectedValue)
			}
			if expectedValue {
				state = "bool_true"
			} else {
				state = "bool_false"
			}
		} else {
			if activeExists {
				delete(m.activeAlarms, name)
				m.appendEventLocked("alarm_recovered", name, "布尔变量报警恢复正常", value, active.Limit, active.Expected, active.Unit)
			}
			return
		}
	} else {
		n, ok := numericValue(value)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return
		}
		if rule.High != nil && n > *rule.High {
			v := *rule.High
			limit = &v
			state = "high"
			if message == "" {
				message = "变量超过上限"
			}
		} else if rule.Low != nil && n < *rule.Low {
			v := *rule.Low
			limit = &v
			state = "low"
			if message == "" {
				message = "变量低于下限"
			}
		} else {
			if activeExists {
				delete(m.activeAlarms, name)
				m.appendEventLocked("alarm_recovered", name, "变量报警恢复正常", value, active.Limit, active.Expected, active.Unit)
			}
			return
		}
	}

	if !activeExists || active.State != state {
		msg := message
		switch state {
		case "high":
			msg += "（高限）"
		case "low":
			msg += "（低限）"
		case "bool_true":
			msg += "（True报警）"
		case "bool_false":
			msg += "（False报警）"
		}
		eventType := "alarm_" + state
		m.appendEventLocked(eventType, name, msg, value, limit, expected, rule.Unit)
	}
	m.activeAlarms[name] = AlarmStatus{Tag: name, Value: value, State: state, Limit: limit, Expected: expected, Unit: rule.Unit, Message: message, Timestamp: now}
}

func (m *Manager) findAlarmRuleLocked(tag string) (model.AlarmRule, bool) {
	for _, rule := range m.alarmConfig.Alarms {
		if rule.Tag == tag {
			return rule, true
		}
	}
	return model.AlarmRule{}, false
}

func boolValue(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	default:
		return false, false
	}
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func downsampleSamples(samples []Sample, maxPoints int) []Sample {
	if maxPoints <= 0 || len(samples) <= maxPoints {
		return samples
	}
	result := make([]Sample, 0, maxPoints)
	step := float64(len(samples)-1) / float64(maxPoints-1)
	last := -1
	for i := 0; i < maxPoints; i++ {
		idx := int(float64(i) * step)
		if idx == last {
			continue
		}
		result = append(result, samples[idx])
		last = idx
	}
	if len(result) > maxPoints {
		result = result[:maxPoints]
	}
	return result
}

func (m *Manager) Get(name string) (Point, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.points[name]
	return p, ok
}

func (m *Manager) All() []Point {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Point, 0, len(m.points))
	for _, p := range m.points {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (m *Manager) HistoryRange(name string, start, end time.Time) ([]Sample, error) {
	m.mu.RLock()
	if !m.historyGlobalEnabled || !m.historyEnabled[name] {
		m.mu.RUnlock()
		return []Sample{}, nil
	}
	store := m.historyStore
	inMemory := append([]Sample(nil), m.history[name]...)
	maxQueryPoints := m.historyQueryMaxPoints
	m.mu.RUnlock()

	if !start.Before(end) {
		return []Sample{}, nil
	}
	if store != nil {
		return store.QueryRange(name, start, end, maxQueryPoints)
	}
	result := make([]Sample, 0, len(inMemory))
	for _, sample := range inMemory {
		if !sample.Timestamp.Before(start) && sample.Timestamp.Before(end) {
			result = append(result, sample)
		}
	}
	if len(result) > maxQueryPoints && maxQueryPoints > 0 {
		result = downsampleSamples(result, maxQueryPoints)
	}
	return result, nil
}

func (m *Manager) History(name string, limit int) ([]Sample, error) {
	m.mu.RLock()
	if !m.historyGlobalEnabled || !m.historyEnabled[name] {
		m.mu.RUnlock()
		return []Sample{}, nil
	}
	store := m.historyStore
	inMemory := append([]Sample(nil), m.history[name]...)
	m.mu.RUnlock()

	if limit <= 0 {
		limit = 60
	}
	if limit > 5000 {
		limit = 5000
	}
	if store != nil {
		return store.Query(name, limit)
	}
	if limit < len(inMemory) {
		inMemory = inMemory[len(inMemory)-limit:]
	}
	return inMemory, nil
}

func (m *Manager) AlarmEvents(tag string, start, end time.Time) ([]Event, error) {
	m.mu.RLock()
	store := m.historyStore
	inMemory := append([]Event(nil), m.events...)
	m.mu.RUnlock()
	if !start.Before(end) {
		return []Event{}, nil
	}
	if store != nil {
		return store.QueryAlarmEvents(tag, start, end)
	}
	result := make([]Event, 0)
	for _, event := range inMemory {
		if !strings.HasPrefix(event.Type, "alarm_") {
			continue
		}
		if strings.TrimSpace(tag) != "" && event.Tag != tag {
			continue
		}
		if event.Timestamp.Before(start) || !event.Timestamp.Before(end) {
			continue
		}
		result = append(result, event)
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

func (m *Manager) Events(limit int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit > len(m.events) {
		limit = len(m.events)
	}
	start := len(m.events) - limit
	result := append([]Event(nil), m.events[start:]...)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (m *Manager) appendEventLocked(eventType, tag, message string, value any, limit *float64, expected *bool, unit string) {
	if message == "" {
		message = eventType
	}
	m.nextEventID++
	event := Event{
		ID:        m.nextEventID,
		Timestamp: time.Now(),
		Type:      eventType,
		Tag:       tag,
		Message:   message,
		Value:     value,
		Limit:     limit,
		Expected:  expected,
		Unit:      unit,
	}
	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[len(m.events)-m.maxEvents:]
	}
	if strings.HasPrefix(eventType, "alarm_") && m.historyStore != nil {
		if err := m.historyStore.AppendAlarmEvent(event); err != nil {
			log.Printf("alarm event persistence failed: tag=%s type=%s err=%v", tag, eventType, err)
		}
	}
}

// Kept as a small function so the manager does not import a logging framework.
func logHistoryError(tag string, err error) {
	log.Printf("history persistence failed: tag=%s err=%v", tag, err)
}
