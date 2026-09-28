package io

import (
	"testing"
	"time"

	"modbus-tcp-driver-v3/internal/model"
)

func TestManagerHistorySamplesOncePerMinute(t *testing.T) {
	m := NewManager()
	m.RegisterPointDefinitions([]PointDefinition{{Name: "P1", DataType: model.Float32, History: true}})
	m.SetHistoryPolicy(true, time.Minute, 2000)
	m.SetHistoryConfig(map[string]bool{"P1": true})

	base := time.Date(2026, 9, 20, 12, 0, 5, 0, time.UTC)
	m.setAt(base, "P1", 1.0, Good, "")
	m.setAt(base.Add(10*time.Second), "P1", 2.0, Good, "")
	m.setAt(base.Add(50*time.Second), "P1", 3.0, Good, "")
	m.setAt(base.Add(70*time.Second), "P1", 4.0, Good, "")

	samples, err := m.History("P1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 {
		t.Fatalf("history sample count = %d, want 2 minute samples", len(samples))
	}
	if got := samples[0].Value; got != 1.0 {
		t.Fatalf("first minute sample = %v, want 1.0", got)
	}
	if got := samples[1].Value; got != 4.0 {
		t.Fatalf("second minute sample = %v, want 4.0", got)
	}
}
