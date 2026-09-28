package history

import (
	"testing"
	"time"

	ioData "modbus-tcp-driver-v3/internal/io"
)

func TestAlarmEventRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir + "/history.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	expected := true
	limit := 50.0
	event := ioData.Event{
		Timestamp: time.Now().UTC(),
		Type:      "alarm_high",
		Tag:       "P1",
		Message:   "超过上限",
		Value:     52.5,
		Limit:     &limit,
		Unit:      "kPa",
	}
	if err := store.AppendAlarmEvent(event); err != nil {
		t.Fatal(err)
	}

	event2 := ioData.Event{
		Timestamp: event.Timestamp.Add(time.Minute),
		Type:      "alarm_bool_true",
		Tag:       "Run",
		Message:   "True报警",
		Value:     true,
		Expected:  &expected,
	}
	if err := store.AppendAlarmEvent(event2); err != nil {
		t.Fatal(err)
	}

	got, err := store.QueryAlarmEvents("P1", event.Timestamp.Add(-time.Second), event.Timestamp.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Tag != "P1" || got[0].Type != "alarm_high" {
		t.Fatalf("unexpected alarm history: %#v", got)
	}
	if got[0].Limit == nil || *got[0].Limit != limit {
		t.Fatalf("unexpected limit: %#v", got[0].Limit)
	}
	if got[0].Value != 52.5 {
		t.Fatalf("unexpected value: %#v", got[0].Value)
	}
}
