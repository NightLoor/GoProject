package io

import (
	"testing"

	"modbus-tcp-driver-v3/internal/model"
)

func TestManagerHistoryAndEvents(t *testing.T) {
	m := NewManager()
	m.SetHistoryConfig(map[string]bool{"Temperature": true})
	m.Set("Temperature", 20.0, Good, "")
	m.Set("Temperature", nil, Bad, "read timeout")
	m.Set("Temperature", nil, Bad, "read timeout")
	m.Set("Temperature", 21.0, Good, "")

	history, err := m.History("Temperature", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1 minute sample", len(history))
	}

	events := m.Events(10)
	if len(events) != 2 {
		t.Fatalf("event length = %d, want 2", len(events))
	}
	if events[0].Type != "quality_recovered" || events[1].Type != "quality_bad" {
		t.Fatalf("unexpected event order/types: %#v", events)
	}
}

func TestAlarmHighAndRecovery(t *testing.T) {
	high := 10.0
	m := NewManager()
	m.SetAlarmConfig(model.AlarmConfig{Enabled: true, Alarms: []model.AlarmRule{{Tag: "P1", Enabled: true, High: &high, Message: "测试报警"}}})
	m.Set("P1", 12.5, Good, "")
	alarms := m.Alarms()
	if len(alarms) != 1 || alarms[0].State != "high" || alarms[0].Limit == nil || *alarms[0].Limit != high {
		t.Fatalf("unexpected active alarms: %#v", alarms)
	}
	events := m.Events(10)
	if len(events) == 0 || events[0].Type != "alarm_high" || events[0].Value != 12.5 {
		t.Fatalf("unexpected alarm event: %#v", events)
	}
	m.Set("P1", 9.5, Good, "")
	if got := len(m.Alarms()); got != 0 {
		t.Fatalf("expected alarm recovery, got %d active alarms", got)
	}
	events = m.Events(10)
	if events[0].Type != "alarm_recovered" {
		t.Fatalf("expected recovery event first, got %#v", events[0])
	}
}

func TestAlarmLow(t *testing.T) {
	low := 5.0
	m := NewManager()
	m.SetAlarmConfig(model.AlarmConfig{Enabled: true, Alarms: []model.AlarmRule{{Tag: "P1", Enabled: true, Low: &low}}})
	m.Set("P1", 3.0, Good, "")
	alarms := m.Alarms()
	if len(alarms) != 1 || alarms[0].State != "low" {
		t.Fatalf("unexpected low alarm: %#v", alarms)
	}
}

func TestBoolAlarmTrueAndFalse(t *testing.T) {
	trueAlarm := true
	m := NewManager()
	m.SetAlarmConfig(model.AlarmConfig{Enabled: true, Alarms: []model.AlarmRule{{
		Tag: "Bool1", Enabled: true, DataType: model.Bool, AlarmValue: &trueAlarm,
	}}})
	m.Set("Bool1", true, Good, "")
	alarms := m.Alarms()
	if len(alarms) != 1 || alarms[0].State != "bool_true" || alarms[0].Expected == nil || !*alarms[0].Expected {
		t.Fatalf("unexpected true alarm: %#v", alarms)
	}
	m.Set("Bool1", false, Good, "")
	if got := len(m.Alarms()); got != 0 {
		t.Fatalf("expected bool alarm recovery, got %d", got)
	}
	events := m.Events(10)
	if len(events) < 2 || events[0].Type != "alarm_recovered" || events[1].Type != "alarm_bool_true" {
		t.Fatalf("unexpected bool alarm events: %#v", events)
	}
}

func TestRegisterPointDefinitionsMakesIOManagerVariablesVisible(t *testing.T) {
	m := NewManager()
	m.RegisterPointDefinitions([]PointDefinition{{Name: "P1", DataType: model.Float32, Description: "测试点", History: true}})
	p, ok := m.Get("P1")
	if !ok {
		t.Fatal("expected registered point")
	}
	if p.DataType != model.Float32 || p.Description != "测试点" || !p.History || p.Quality != Bad {
		t.Fatalf("unexpected registered point: %#v", p)
	}
}
