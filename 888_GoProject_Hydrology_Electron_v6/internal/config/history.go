package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"modbus-tcp-driver-v3/internal/model"
)

func DefaultHistoryConfig() model.HistoryConfig {
	return model.HistoryConfig{
		Enabled:               true,
		SampleIntervalMinutes: 1,
		RetentionDays:         730,
		ArchiveEnabled:        true,
		ArchiveDir:            "data/archive",
		ArchiveRetentionDays:  3650,
		CleanupIntervalHours:  24,
		QueryMaxPoints:        2000,
	}
}

func LoadHistoryJSON(path string) (model.HistoryConfig, error) {
	cfg := DefaultHistoryConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	var raw model.HistoryConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("decode history config %s: %w", path, err)
	}
	if raw.ArchiveDir == "" {
		raw.ArchiveDir = cfg.ArchiveDir
	}
	if raw.SampleIntervalMinutes <= 0 {
		raw.SampleIntervalMinutes = cfg.SampleIntervalMinutes
	}
	if raw.RetentionDays <= 0 {
		raw.RetentionDays = cfg.RetentionDays
	}
	if raw.ArchiveRetentionDays <= 0 {
		raw.ArchiveRetentionDays = cfg.ArchiveRetentionDays
	}
	if raw.CleanupIntervalHours <= 0 {
		raw.CleanupIntervalHours = cfg.CleanupIntervalHours
	}
	if raw.QueryMaxPoints <= 0 {
		raw.QueryMaxPoints = cfg.QueryMaxPoints
	}
	if strings.TrimSpace(raw.ArchiveDir) == "" {
		raw.ArchiveDir = cfg.ArchiveDir
	}
	if raw.ArchiveRetentionDays < raw.RetentionDays {
		raw.ArchiveRetentionDays = raw.RetentionDays
	}
	return raw, nil
}
