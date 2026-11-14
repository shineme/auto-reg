package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearRunStateModes(t *testing.T) {
	outputDir := t.TempDir()
	batchDir := filepath.Join(outputDir, "resume-200")
	accountDir := filepath.Join(batchDir, "account-001")
	if err := os.MkdirAll(accountDir, 0o755); err != nil {
		t.Fatal(err)
	}
	statePaths := []string{
		filepath.Join(outputDir, "supervisor.status.json"),
		filepath.Join(outputDir, "supervisor.progress.json"),
		filepath.Join(outputDir, "supervisor.lock"),
		filepath.Join(outputDir, "risk-state.json"),
		filepath.Join(outputDir, "resume-ledger.json"),
		filepath.Join(batchDir, "risk-control.json"),
		filepath.Join(batchDir, "summary.json"),
		filepath.Join(accountDir, "result.json"),
	}
	for _, statePath := range statePaths {
		if err := os.WriteFile(statePath, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := clearRunState(outputDir, false); err != nil {
		t.Fatalf("clearRunState(false): %v", err)
	}
	for _, statePath := range []string{
		filepath.Join(outputDir, "supervisor.status.json"),
		filepath.Join(outputDir, "supervisor.progress.json"),
		filepath.Join(outputDir, "supervisor.lock"),
		filepath.Join(outputDir, "risk-state.json"),
		filepath.Join(batchDir, "risk-control.json"),
	} {
		if _, err := os.Stat(statePath); !os.IsNotExist(err) {
			t.Fatalf("state file still exists: %s", statePath)
		}
	}
	for _, statePath := range []string{
		filepath.Join(outputDir, "resume-ledger.json"),
		filepath.Join(batchDir, "summary.json"),
		filepath.Join(accountDir, "result.json"),
	} {
		if _, err := os.Stat(statePath); err != nil {
			t.Fatalf("history should be preserved: %s: %v", statePath, err)
		}
	}
	if _, err := clearRunState(outputDir, true); err != nil {
		t.Fatalf("clearRunState(true): %v", err)
	}
	for _, statePath := range statePaths {
		if _, err := os.Stat(statePath); !os.IsNotExist(err) {
			t.Fatalf("history not removed: %s", statePath)
		}
	}
}
