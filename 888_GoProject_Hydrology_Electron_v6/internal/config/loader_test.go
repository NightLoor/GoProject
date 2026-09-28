package config

import (
	"strings"
	"testing"

	"modbus-tcp-driver-v3/internal/model"
)

func TestValidateConfigRejectsDuplicateTags(t *testing.T) {
	cfg := model.Config{Devices: []model.DeviceConfig{
		{Name: "dev1", Address: "127.0.0.1:502", Tags: []model.TagConfig{{Name: "Run"}}},
		{Name: "dev2", Address: "127.0.0.2:502", Tags: []model.TagConfig{{Name: "Run"}}},
	}}

	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate tag Run") {
		t.Fatalf("ValidateConfig error = %v, want duplicate tag error", err)
	}
}
