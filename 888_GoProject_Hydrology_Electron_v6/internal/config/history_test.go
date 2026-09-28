package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHistoryJSONDefaultsAndValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	data := []byte(`{
      "enabled": true,
      "sample_interval_minutes": 1,
      "retention_days": 730,
      "archive_enabled": true,
      "archive_dir": "data/archive",
      "archive_retention_days": 3650,
      "cleanup_interval_hours": 24,
      "query_max_points": 2000
    }`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadHistoryJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SampleIntervalMinutes != 1 || cfg.RetentionDays != 730 || cfg.QueryMaxPoints != 2000 {
		t.Fatalf("unexpected history config: %#v", cfg)
	}
	if !cfg.ArchiveEnabled || cfg.ArchiveRetentionDays != 3650 {
		t.Fatalf("unexpected archive policy: %#v", cfg)
	}
}
