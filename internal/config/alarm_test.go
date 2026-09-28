package config

import (
	"os"
	"path/filepath"
	"testing"

	"modbus-tcp-driver-v3/internal/model"
)

func testAlarmModel() model.Config {
	return model.Config{
		OPCDAEnabled:    true,
		OPCDAEnabledSet: true,
		OPCDADevices: []model.OPCDADeviceConfig{{
			Name: "opcda_server_1",
			Tags: []model.OPCDATagConfig{
				{Name: "Bool1", DataType: model.Bool, Description: "布尔点"},
				{Name: "Float1", DataType: model.Float32, Description: "浮点点"},
			},
		}},
	}
}

func TestLoadAlarmCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Alarm.csv")
	content := "tag,data_type,enabled,alarm_value,low,high,unit,message\nBool1,bool,true,true,,,,布尔报警\nFloat1,float32,true,,1.5,10.5,kPa,浮点报警\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadAlarmCSV(path, testAlarmModel())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Alarms) != 2 {
		t.Fatalf("unexpected alarm rule count: %d", len(cfg.Alarms))
	}
	if cfg.Alarms[0].Tag != "Bool1" && cfg.Alarms[1].Tag != "Bool1" {
		t.Fatal("bool rule missing")
	}
}

func TestLoadAlarmCSVRejectsBadLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Alarm.csv")
	content := "tag,data_type,enabled,alarm_value,low,high,unit,message\nFloat1,float32,true,,20,10,kPa,浮点报警\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAlarmCSV(path, testAlarmModel()); err == nil {
		t.Fatal("expected invalid limit error")
	}
}

func TestSaveAlarmCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Alarm.csv")
	v := true
	high := 20.0
	cfg := model.AlarmConfig{Enabled: true, Alarms: []model.AlarmRule{
		{Tag: "Bool1", DataType: model.Bool, Enabled: true, AlarmValue: &v, Message: "布尔报警"},
		{Tag: "Float1", DataType: model.Float32, Enabled: true, High: &high, Unit: "kPa", Message: "浮点报警"},
	}}
	if err := SaveAlarmCSV(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAlarmCSV(path, testAlarmModel())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Alarms) != 2 {
		t.Fatalf("unexpected saved rules: %#v", got)
	}
}
