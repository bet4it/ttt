package fff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFffEngine(t *testing.T) {
	if !Available() {
		t.Skip("fff CGO is not available")
	}

	// Use the current repo root
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to get repo root: %v", err)
	}

	inst, err := NewInstance(repoRoot, Options{
		Watch:                 false,
		EnableContentIndexing: true,
	})
	if err != nil {
		t.Fatalf("failed to create fff instance: %v", err)
	}
	defer inst.Close()

	if !inst.WaitForScan(5 * time.Second) {
		t.Log("initial scan wait timed out, continuing")
	}

	// Test SearchFiles
	searchRes, err := inst.SearchFiles("search_widget", SearchFileOptions{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("SearchFiles failed: %v", err)
	}
	if searchRes.TotalMatched == 0 {
		t.Errorf("expected matches for 'search_widget', got 0")
	} else {
		t.Logf("SearchFiles matched %d files (total: %d)", searchRes.TotalMatched, searchRes.TotalFiles)
		for _, f := range searchRes.Files {
			t.Logf("  match: %s", f.RelativePath)
		}
	}

	// Test LiveGrep
	grepRes, err := inst.LiveGrep("streamFiles", LiveGrepOptions{
		Mode:      GrepModePlainText,
		PageLimit: 10,
	})
	if err != nil {
		t.Fatalf("LiveGrep failed: %v", err)
	}
	if grepRes.TotalMatched == 0 {
		t.Errorf("expected grep matches for 'streamFiles', got 0")
	} else {
		t.Logf("LiveGrep matched %d lines", grepRes.TotalMatched)
		for _, m := range grepRes.Matches {
			t.Logf("  line: %s:%d (col: %d) -> %s", m.RelativePath, m.LineNumber, m.Col, m.LineContent)
		}
	}

	// Test ListFiles
	files, err := inst.ListFiles(100)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(files) == 0 {
		t.Errorf("expected indexed files from ListFiles, got 0")
	} else {
		t.Logf("ListFiles returned %d files", len(files))
	}
}

func TestManager(t *testing.T) {
	if !Available() {
		t.Skip("fff CGO is not available")
	}

	tmpDir, err := os.MkdirTemp("", "fff_test_mgr_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := DefaultManager()
	inst, err := mgr.GetOrInit(tmpDir)
	if err != nil {
		t.Fatalf("GetOrInit failed: %v", err)
	}
	if inst == nil {
		t.Fatal("expected non-nil instance")
	}

	// Fetching same dir returns same instance
	inst2, err := mgr.GetOrInit(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if inst != inst2 {
		t.Error("expected same instance for same directory")
	}
}
