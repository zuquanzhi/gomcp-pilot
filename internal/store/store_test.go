package store

import (
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestStore(t *testing.T) {
	// Setup temp db path
	tempDir, err := os.MkdirTemp("", "gomcp_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the user home dir by changing the global variable logic?
	// The InitStore function uses os.UserHomeDir().
	// We should probably modify InitStore to take a path or be more testable.
	// For now, let's just modify the variable if we can or use a hack.
	// Actually, InitStore hardcodes ~/.gomcp.
	// Let's modify InitStore to accept a DB path option or check an env var.
	// OR, for this test, let's just use the current implementation but realize it might fail if it tries to write to real home dir.
	// Wait, I can set HOME env var for the test process?
	// Yes, os.Setenv("HOME", tempDir) might work on Unix.

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tempDir)

	if err := InitStore(); err != nil {
		t.Fatalf("InitStore failed: %v", err)
	}
	defer Close()

	// 1. Record a call
	duration := 123 * time.Millisecond
	err = RecordCall("test_upstream", "list_files", `{"path":"/"}`, "success", "", duration)
	if err != nil {
		t.Fatalf("RecordCall failed: %v", err)
	}

	// 2. Query calls
	records, err := GetRecentCalls(10)
	if err != nil {
		t.Fatalf("GetRecentCalls failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Upstream != "test_upstream" {
		t.Errorf("expected upstream test_upstream, got %s", rec.Upstream)
	}
	if rec.Tool != "list_files" {
		t.Errorf("expected tool list_files, got %s", rec.Tool)
	}
	if rec.DurationMs != 123 {
		t.Errorf("expected duration 123, got %d", rec.DurationMs)
	}
}
