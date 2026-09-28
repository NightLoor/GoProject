package modbus

import (
	"sort"
	"sync"

	"modbus-tcp-driver-v3/internal/driver"
)

type statusStore struct {
	mu     sync.RWMutex
	values map[string]driver.DeviceStatus
}

func newStatusStore() *statusStore {
	return &statusStore{values: make(map[string]driver.DeviceStatus)}
}

func (s *statusStore) set(name string, update func(*driver.DeviceStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.values[name]
	update(&status)
	s.values[name] = status
}

func (s *statusStore) all() []driver.DeviceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]driver.DeviceStatus, 0, len(s.values))
	for _, v := range s.values {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
