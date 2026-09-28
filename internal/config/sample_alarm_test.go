package config

import (
	"path/filepath"
	"testing"
)

func TestSampleAlarmConfigLoadsWithSuppliedConfig(t *testing.T) {
	root := filepath.Join("..", "..", "configs")
	modbus, err := LoadJSON(filepath.Join(root, "modbus.json"))
	if err != nil {
		t.Fatal(err)
	}
	opcda, err := LoadJSON(filepath.Join(root, "opcda.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := modbus
	cfg.OPCDAEnabled = opcda.OPCDAEnabled
	cfg.OPCDAEnabledSet = opcda.OPCDAEnabledSet
	cfg.OPCDADevices = opcda.OPCDADevices
	if err := LoadTagsCSV(filepath.Join(root, "Tags.csv"), &cfg); err != nil {
		t.Fatal(err)
	}
	alarm, err := LoadAlarmCSV(filepath.Join(root, "Alarm.csv"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Alarm = alarm
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if len(alarm.Alarms) != 30 {
		t.Fatalf("alarm rule count=%d, want 30", len(alarm.Alarms))
	}
}
