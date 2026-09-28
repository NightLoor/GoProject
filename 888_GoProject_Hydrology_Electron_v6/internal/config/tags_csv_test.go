package config

import (
	"os"
	"path/filepath"
	"testing"

	"modbus-tcp-driver-v3/internal/model"
)

func TestLoadTagsCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "Tags.csv")
	content := "protocol,device,tag_name,address,data_type,writable,slave_id,register_type,scale,offset,byte_order,description,history\n" +
		"Modbus,m1,R1,40001,int16,true,1,holding_register,1,0,,r1,true\n" +
		"OPCDA,o1,T1,Channel1.Device1.T1,bool,true,,,1,0,,t1,false\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := model.Config{
		Devices:      []model.DeviceConfig{{Name: "m1", Address: "127.0.0.1:502"}},
		OPCDAEnabled: true,
		OPCDADevices: []model.OPCDADeviceConfig{{Name: "o1", ProgID: "Test.ProgID", Node: "localhost"}},
	}
	if err := LoadTagsCSV(csvPath, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Devices[0].Tags) != 1 || cfg.Devices[0].Tags[0].Name != "R1" {
		t.Fatalf("unexpected modbus tags: %#v", cfg.Devices[0].Tags)
	}
	if len(cfg.OPCDADevices[0].Tags) != 1 || cfg.OPCDADevices[0].Tags[0].Name != "T1" {
		t.Fatalf("unexpected opcda tags: %#v", cfg.OPCDADevices[0].Tags)
	}
}
