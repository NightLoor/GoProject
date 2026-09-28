package config

import (
	"path/filepath"
	"testing"
)

func TestSampleTagsCSVLoadsAllEnabledProtocols(t *testing.T) {
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
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}

	if got := len(cfg.Devices); got != 1 {
		t.Fatalf("modbus device count=%d, want 1", got)
	}
	if got := len(cfg.Devices[0].Tags); got != 0 {
		t.Fatalf("modbus tag count=%d, want 0 because Modbus is disabled", got)
	}
	if got := len(cfg.OPCDADevices); got != 1 {
		t.Fatalf("opcda device count=%d, want 1", got)
	}
	if got := len(cfg.OPCDADevices[0].Tags); got != 30 {
		t.Fatalf("opcda tag count=%d, want 30", got)
	}
	if got := cfg.OPCDADevices[0].Tags[0].Description; got == "" {
		t.Fatal("expected OPC DA tag description to load from CSV")
	}
	if !cfg.OPCDADevices[0].Tags[0].History {
		t.Fatal("expected OPC DA tag history to be enabled from CSV")
	}
}
