package io

import (
	"testing"
	"time"
)

func TestManagerHistoryDisabledDoesNotRecord(t *testing.T) {
	m := NewManager()
	m.SetHistoryConfig(map[string]bool{"P1": false})
	m.Set("P1", 12.3, Good, "")
	history, err := m.History("P1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history length = %d, want 0", len(history))
	}
	point, ok := m.Get("P1")
	if !ok || point.History {
		t.Fatalf("unexpected point history flag: %#v", point)
	}
}

func TestManagerHistoryEnabledRecords(t *testing.T) {
	m := NewManager()
	m.SetHistoryConfig(map[string]bool{"P1": true})
	m.Set("P1", 12.3, Good, "")
	history, err := m.History("P1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
}

func TestManagerHistoryRange(t *testing.T) {
	m := NewManager()
	m.SetHistoryConfig(map[string]bool{"P1": true})
	base := time.Now().Add(-4 * time.Hour)
	m.points["P1"] = Point{Name: "P1", Value: 10.0, Quality: Good, Timestamp: base, History: true}
	m.history["P1"] = []Sample{
		{Timestamp: base, Value: 10.0, Quality: Good},
		{Timestamp: base.Add(2 * time.Hour), Value: 12.0, Quality: Good},
		{Timestamp: base.Add(4 * time.Hour), Value: 14.0, Quality: Good},
	}
	values, err := m.HistoryRange("P1", base.Add(time.Hour), base.Add(5*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("history range length = %d, want 2", len(values))
	}
}
