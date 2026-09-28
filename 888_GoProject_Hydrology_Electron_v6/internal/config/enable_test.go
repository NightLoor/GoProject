package config

import (
	"os"
	"path/filepath"
	"testing"

	"modbus-tcp-driver-v3/internal/model"
)

func TestProtocolEnabledFlags(t *testing.T) {
	root := filepath.Join("..", "..", "configs")
	modbus, err := LoadJSON(filepath.Join(root, "modbus.json"))
	if err != nil {
		t.Fatal(err)
	}
	if modbus.ModbusEnabled {
		t.Fatal("modbus should be disabled in the supplied config")
	}
	if len(modbus.Devices) != 1 {
		t.Fatalf("expected 1 modbus device, got %d", len(modbus.Devices))
	}

	opcda, err := LoadJSON(filepath.Join(root, "opcda.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !opcda.OPCDAEnabled {
		t.Fatal("opcda should be enabled in sample config")
	}
	if len(opcda.OPCDADevices) != 1 {
		t.Fatalf("expected 1 opcda device, got %d", len(opcda.OPCDADevices))
	}
}

func TestDisabledProtocolIgnoresTags(t *testing.T) {
	cfg := model.Config{ModbusEnabled: false, ModbusEnabledSet: true, Devices: []model.DeviceConfig{{Name: "m1", Address: "127.0.0.1:502"}}}
	path := filepath.Join(t.TempDir(), "Tags.csv")
	if err := os.WriteFile(path, []byte("protocol,device,tag_name,address,data_type,writable,slave_id,scale,offset,byte_order\nMODBUS,m1,Tag1,00001,bool,false,1,1,0,\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := LoadTagsCSV(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Devices[0].Tags) != 0 {
		t.Fatalf("expected disabled Modbus tags to be ignored, got %d", len(cfg.Devices[0].Tags))
	}
}
