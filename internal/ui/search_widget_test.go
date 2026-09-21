package ui

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/fff"
)

func TestRgFailureIgnoresNoMatches(t *testing.T) {
	// rg exits 1 when it finds nothing, which is an empty result, not a failure.
	err := exec.Command("sh", "-c", "exit 1").Run()
	if err == nil {
		t.Fatal("expected a non-nil error from exit 1")
	}
	if got := rgFailure(err); got != "" {
		t.Errorf("rgFailure(exit 1) = %q, want empty", got)
	}
}

func TestRgFailureReportsRealFailures(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 2").Run()
	if err == nil {
		t.Fatal("expected a non-nil error from exit 2")
	}
	if got := rgFailure(err); got == "" {
		t.Error("rgFailure(exit 2) = empty, want a message")
	}

	if got := rgFailure(errors.New("boom")); got != "search failed: boom" {
		t.Errorf("rgFailure(boom) = %q", got)
	}
	if got := rgFailure(nil); got != "" {
		t.Errorf("rgFailure(nil) = %q, want empty", got)
	}
}

func TestApplyBatchSurfacesError(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 7

	s.ApplyBatch(&SearchBatch{Gen: 7, Done: true, Error: "ripgrep (rg) not found"})
	if s.Error != "ripgrep (rg) not found" {
		t.Fatalf("Error = %q, want the rg message", s.Error)
	}
	if s.Searching {
		t.Error("expected Searching to be cleared")
	}
}

func TestApplyBatchIgnoresStaleGeneration(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 8

	s.ApplyBatch(&SearchBatch{Gen: 7, Done: true, Error: "stale"})
	if s.Error != "" {
		t.Errorf("Error = %q, want empty for a stale batch", s.Error)
	}
}

// A partial batch must not wipe a message set by the run that follows it.
func TestApplyBatchKeepsErrorUntilDone(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 3
	s.Error = "previous"

	s.ApplyBatch(&SearchBatch{Gen: 3})
	if s.Error != "previous" {
		t.Errorf("Error = %q, want it untouched by a partial batch", s.Error)
	}

	s.ApplyBatch(&SearchBatch{Gen: 3, Done: true})
	if s.Error != "" {
		t.Errorf("Error = %q, want cleared by a successful final batch", s.Error)
	}
}

func TestSearchStartClearsError(t *testing.T) {
	s := NewSearchWidget()
	s.Error = "ripgrep (rg) not found"
	s.Input.Text = ""

	s.runSearchSync()
	if s.Error != "" {
		t.Errorf("Error = %q, want cleared when a new search starts", s.Error)
	}
}

func TestSearchWidgetFffIntegration(t *testing.T) {
	if !fff.Available() {
		t.Skip("fff CGO is not available")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	s := NewSearchWidget()
	s.WorkDirs = []string{repoRoot}
	s.Input.Text = "streamFilesFff"

	var lastBatch *SearchBatch
	s.PostBatch = func(b *SearchBatch) {
		lastBatch = b
		s.ApplyBatch(b)
	}

	var groups []SearchFileGroup
	s.streamFilesFff(context.Background(), 1, &groups)
	if lastBatch == nil || !lastBatch.Done {
		t.Fatal("expected done batch from streamFilesFff")
	}
	if len(groups) == 0 {
		t.Fatal("expected search matches for 'streamFilesFff'")
	}

	found := false
	for _, g := range groups {
		for _, m := range g.Matches {
			if strings.Contains(m.LineText, "streamFilesFff") {
				found = true
				if m.LineNum <= 0 {
					t.Errorf("expected positive line number, got %d", m.LineNum)
				}
				if m.ColStart < 0 || m.ColEnd <= m.ColStart {
					t.Errorf("invalid match column range: %d..%d", m.ColStart, m.ColEnd)
				}
			}
		}
	}
	if !found {
		t.Error("expected to find streamFilesFff in search matches")
	}
}

func TestSearchWidgetEngineRouting(t *testing.T) {
	s := NewSearchWidget()
	s.Engine = "ripgrep"
	s.searchGen = 1
	var lastBatch *SearchBatch
	s.PostBatch = func(b *SearchBatch) {
		lastBatch = b
		s.ApplyBatch(b)
	}

	var groups []SearchFileGroup
	s.streamFiles(context.Background(), 1, &groups)
	if lastBatch == nil || !lastBatch.Done {
		t.Fatal("expected done batch from streamFiles with ripgrep")
	}
}
