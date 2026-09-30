package app

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestCommitHistoryHeightRestoresAndPersists(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	settings.Sidebar.CommitHistoryHeight = 17
	a := buildTestApp(t, settings)
	if a.Changes.Split.BottomH != 17 || a.Changes.Split.BottomRatio != 0 {
		t.Fatalf("restored split = height %d ratio %v, want height 17 ratio 0", a.Changes.Split.BottomH, a.Changes.Split.BottomRatio)
	}

	a.Changes.Split.BottomH = 12
	a.persistCommitHistoryHeight()
	if got := a.State.CommitHistoryHeight; got != 12 {
		t.Fatalf("commitHistoryHeight = %d, want 12", got)
	}
	if got := config.LoadState().CommitHistoryHeight; got != 12 {
		t.Fatalf("persisted commitHistoryHeight = %d, want 12", got)
	}
}

func TestSidebarWidthRestoresAndPersists(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	settings := config.DefaultSettings()
	settings.Sidebar.Width = 22
	a := buildTestApp(t, settings)
	if a.SplitPanel.DividerPos != 22 {
		t.Fatalf("restored sidebar width = %d, want 22", a.SplitPanel.DividerPos)
	}

	a.SetSidebarWidth(18)
	if got := a.State.SidebarWidth; got != 18 {
		t.Fatalf("sidebar width = %d, want 18", got)
	}
	if got := config.LoadState().SidebarWidth; got != 18 {
		t.Fatalf("persisted sidebar width = %d, want 18", got)
	}
}

func TestSidebarDraggedClosedRestoresUsableWidth(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	a := buildTestApp(t, config.DefaultSettings())
	a.SetSidebarWidth(18)
	for w := 5; w >= 0; w-- {
		a.resizeSidebar(w)
	}
	a.persistSidebarLayout()
	if got := config.LoadState(); got.SidebarWidth != 18 || !got.SidebarHidden {
		t.Fatalf("persisted state = %+v, want width 18 and hidden", got)
	}

	if err := config.SaveState(config.State{SidebarWidth: 2}); err != nil {
		t.Fatal(err)
	}
	a = buildTestApp(t, config.DefaultSettings())
	if a.SplitPanel.DividerPos != ui.DefaultSidebarWidth {
		t.Fatalf("restored sidebar width = %d, want %d", a.SplitPanel.DividerPos, ui.DefaultSidebarWidth)
	}
}

// commitTo writes only the fields the form owns. Settings changed elsewhere
// while the tab sat open — including ones the form never shows — must survive.
// Assigning the whole working struct instead would roll them back.
func TestCommitToLeavesUnownedSettingsAlone(t *testing.T) {
	opened := config.DefaultSettings()
	v := &settingsView{working: opened, categories: settingsCategories()}
	v.working.Editor.TabSize = 7

	live := opened
	live.LSP.HoverDelay = 1234
	live.Formatters = map[string]string{"go": "gofmt"}
	live.LSP.Servers = map[string]config.LSPServerConfig{"go": {Command: []string{"gopls"}}}

	v.commitTo(&live)

	if live.Editor.TabSize != 7 {
		t.Errorf("form edit not committed: TabSize = %d, want 7", live.Editor.TabSize)
	}
	if live.LSP.HoverDelay != 1234 {
		t.Errorf("clobbered an unowned setting: HoverDelay = %d, want 1234", live.LSP.HoverDelay)
	}
	if live.Formatters["go"] != "gofmt" {
		t.Error("clobbered the formatters map")
	}
	if _, ok := live.LSP.Servers["go"]; !ok {
		t.Error("clobbered the lsp servers map")
	}
}

// Every field in the table must be reachable through commitTo, or an edit made
// in the form would be silently dropped on Apply.
func TestCommitToCoversEveryField(t *testing.T) {
	for _, s := range []*config.Settings{nil, {Search: config.SearchSettings{Engine: config.SearchEngineRipgrep}}} {
		for _, cat := range settingsCategories(s) {
			for _, f := range cat.Fields {
				switch f.Kind {
				case settingBool:
					if f.GetBool == nil || f.SetBool == nil {
						t.Errorf("%s → %s: bool field missing an accessor", cat.Title, f.Label)
					}
				case settingInt:
					if f.GetInt == nil || f.SetInt == nil {
						t.Errorf("%s → %s: int field missing an accessor", cat.Title, f.Label)
					}
				default:
					if f.GetString == nil || f.SetString == nil {
						t.Errorf("%s → %s: string field missing an accessor", cat.Title, f.Label)
					}
				}
			}
		}
	}
}

func TestDiffContextSettingLivesInAppearance(t *testing.T) {
	found := ""
	count := 0
	for _, category := range settingsCategories() {
		for _, field := range category.Fields {
			if field.Label == "Diff context" {
				found = category.Title
				count++
			}
		}
	}
	if count != 1 || found != "Appearance" {
		t.Fatalf("Diff context count = %d, category = %q; want one under Appearance", count, found)
	}
}

func TestCollapsedDiffEmphasisSettingLivesInAppearance(t *testing.T) {
	found := ""
	count := 0
	for _, category := range settingsCategories() {
		for _, field := range category.Fields {
			if field.Label == "Emphasize collapsed diff rows" {
				found = category.Title
				count++
			}
		}
	}
	if count != 1 || found != "Appearance" {
		t.Fatalf("collapsed diff emphasis count = %d, category = %q; want one under Appearance", count, found)
	}
}

func TestSearchEngineSettingLivesInAdvanced(t *testing.T) {
	hasFff := slices.Contains(config.SearchEngines(), config.SearchEngineFFF)
	found := ""
	count := 0
	for _, category := range settingsCategories() {
		for _, field := range category.Fields {
			if field.Label == "Search engine" {
				found = category.Title
				count++
				if field.Kind != settingEnum {
					t.Fatalf("search engine field kind = %v, want settingEnum", field.Kind)
				}
				items := field.Options()
				if len(items) != 2 || items[0].ID != config.SearchEngineFFF || items[1].ID != config.SearchEngineRipgrep {
					t.Fatalf("search engine options unexpected: %+v", items)
				}
				s := config.DefaultSettings()
				field.SetString(&s, "ripgrep")
				if got := field.GetString(&s); got != "ripgrep" {
					t.Fatalf("search engine getter = %q, want ripgrep", got)
				}
			}
		}
	}
	if hasFff {
		if count != 1 || found != "Advanced" {
			t.Fatalf("search engine setting count = %d, category = %q; want one under Advanced", count, found)
		}
	} else {
		if count != 0 {
			t.Fatalf("search engine setting should not exist when fff is unavailable, found count = %d", count)
		}
	}
}

func TestShowSettingsReopenPreservesPendingWorkingView(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.EditorGroup.OnContentTabClose = func(id string) {
		a.cleanupPluginDetailTab(id)
		a.cleanupSettingsTab(id)
	}
	a.ShowSettings()
	first := a.settingsView
	if first == nil {
		t.Fatal("opening settings did not create a working view")
	}
	first.working.Editor.TabSize = 7

	a.ShowSettings()

	if a.settingsView != first {
		t.Fatalf("reopening settings discarded working view: got %p, want %p", a.settingsView, first)
	}
	if got := a.settingsView.working.Editor.TabSize; got != 7 {
		t.Fatalf("reopening settings discarded pending tab size: got %d, want 7", got)
	}
}

func TestSidebarClosedStateRestores(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	folder := t.TempDir()
	build := func() *App {
		cfg := config.AppConfig{
			Keybindings: config.DefaultKeybindings(),
			Settings:    config.DefaultSettings(),
			Theme:       config.DefaultTheme(),
		}
		borders := BuildBorderSet(cfg.Theme.Borders)
		return BuildAppFromConfig(&cfg, &borders, workspace.New([]string{folder}), nil)
	}

	a := build()
	if !a.Sidebar.Visible {
		t.Fatal("sidebar should start visible with a folder open")
	}
	a.ToggleSidebar()
	if a = build(); a.Sidebar.Visible || a.SplitPanel.ShowLeft {
		t.Fatal("sidebar closed in the previous session should stay closed")
	}

	a.ToggleSidebar()
	if a = build(); !a.Sidebar.Visible {
		t.Fatal("sidebar reopened in the previous session should start visible")
	}

	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	a.ShowEmptyState()
	if a = build(); !a.Sidebar.Visible {
		t.Fatal("hiding the sidebar for the empty state must not persist")
	}
}

func TestPanelSizeRestoresAndPersists(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	a := buildTestApp(t, config.DefaultSettings())
	a.ContentSplit.BottomH = 12
	a.ContentSplit.RightW = 45
	a.persistPanelSize()

	a.ContentSplit.BottomH = 2
	a.ContentSplit.RightW = 10
	a.persistPanelSize()
	if got := config.LoadState(); got.PanelHeight != 12 || got.PanelWidth != 45 {
		t.Fatalf("persisted panel size = %dx%d, want 12x45", got.PanelHeight, got.PanelWidth)
	}

	a = buildTestApp(t, config.DefaultSettings())
	if a.ContentSplit.BottomH != 12 || a.ContentSplit.RightW != 45 {
		t.Fatalf("restored panel size = %dx%d, want 12x45", a.ContentSplit.BottomH, a.ContentSplit.RightW)
	}

	if err := config.SaveState(config.State{PanelHeight: 2, PanelWidth: 10}); err != nil {
		t.Fatal(err)
	}
	a = buildTestApp(t, config.DefaultSettings())
	if a.ContentSplit.BottomH < ui.MinPanelHeight || a.ContentSplit.RightW < ui.MinPanelWidth {
		t.Fatalf("restored tiny panel size = %dx%d", a.ContentSplit.BottomH, a.ContentSplit.RightW)
	}
}

func TestReopeningTinyPanelResetsSize(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.ContentSplit.SetRect(ui.Rect{W: 100, H: 40})
	a.ContentSplit.BottomH = 2
	a.ContentSplit.RightW = 3
	a.ensureUsablePanelSize()
	if a.ContentSplit.BottomH != 20 || a.ContentSplit.RightW != ui.DefaultPanelWidth {
		t.Fatalf("panel size = %dx%d, want 20x%d", a.ContentSplit.BottomH, a.ContentSplit.RightW, ui.DefaultPanelWidth)
	}
}

func TestSearchDebounceSettingOmittedWhenFff(t *testing.T) {
	hasFff := slices.Contains(config.SearchEngines(), config.SearchEngineFFF)

	// When engine is fff (if supported)
	if hasFff {
		s := config.DefaultSettings()
		s.Search.Engine = config.SearchEngineFFF
		for _, cat := range settingsCategories(&s) {
			if cat.Title == "Advanced" {
				for _, f := range cat.Fields {
					if f.Label == "Search debounce (ms)" {
						t.Errorf("Search debounce (ms) should not be present when engine is fff")
					}
				}
			}
		}
	}

	// When engine is ripgrep
	s := config.DefaultSettings()
	s.Search.Engine = config.SearchEngineRipgrep
	foundDebounce := false
	for _, cat := range settingsCategories(&s) {
		if cat.Title == "Advanced" {
			for _, f := range cat.Fields {
				if f.Label == "Search debounce (ms)" {
					foundDebounce = true
				}
			}
		}
	}
	if !foundDebounce {
		t.Errorf("Search debounce (ms) should be present when engine is ripgrep")
	}
}

func TestSearchDebounceDynamicToggle(t *testing.T) {
	if !slices.Contains(config.SearchEngines(), config.SearchEngineFFF) {
		t.Skip("skipping test: fff is not available in this build")
	}

	a := buildTestApp(t, config.DefaultSettings())
	a.ShowSettings()
	v := a.settingsView
	if v == nil {
		t.Fatal("expected settingsView to be open")
	}

	// Initially on fff: debounceRow should not be in advancedStack.Children
	for _, child := range v.advancedStack.Children {
		if child == v.debounceRow {
			t.Fatal("expected debounceRow to not be in advancedStack.Children initially on fff")
		}
	}
	if v.working.Search.Debounce != 0 {
		t.Fatalf("expected initial debounce to be 0 for fff, got %d", v.working.Search.Debounce)
	}

	// Switch to ripgrep
	v.onSearchEngineChanged(config.SearchEngineRipgrep)
	found := false
	for _, child := range v.advancedStack.Children {
		if child == v.debounceRow {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected debounceRow to be added to advancedStack.Children after switching to ripgrep")
	}
	if v.working.Search.Debounce != 350 {
		t.Fatalf("expected debounce to be reset to 350 for ripgrep, got %d", v.working.Search.Debounce)
	}

	// Switch back to fff
	v.onSearchEngineChanged(config.SearchEngineFFF)
	for _, child := range v.advancedStack.Children {
		if child == v.debounceRow {
			t.Fatal("expected debounceRow to be removed from advancedStack.Children after switching to fff")
		}
	}
	if v.working.Search.Debounce != 0 {
		t.Fatalf("expected debounce to be 0 after switching back to fff, got %d", v.working.Search.Debounce)
	}
}
