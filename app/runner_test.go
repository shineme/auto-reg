package main

import (
	"encoding/json"
	"testing"
)

func TestRunnerEventsSerializeAsEmptyArray(t *testing.T) {
	events := newRunner(nil).Events()
	if events == nil {
		t.Fatal("Events() returned nil")
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("json.Marshal(Events()): %v", err)
	}
	if string(encoded) != "[]" {
		t.Fatalf("json.Marshal(Events()) = %s, want []", encoded)
	}
}
