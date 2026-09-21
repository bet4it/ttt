package fff

import (
	"path/filepath"
	"sync"
	"time"
)

// GrepMode specifies the grep search mode.
type GrepMode uint8

const (
	GrepModePlainText GrepMode = 0
	GrepModeRegex     GrepMode = 1
	GrepModeFuzzy     GrepMode = 2
)

// MatchRange represents byte offsets within a matched line.
type MatchRange struct {
	Start int
	End   int
}

// GrepMatch represents a single line match.
type GrepMatch struct {
	RelativePath string
	FileName     string
	GitStatus    string
	LineContent  string
	LineNumber   int
	Col          int
	MatchRanges  []MatchRange
}

// GrepResult holds the list of matches and statistics.
type GrepResult struct {
	Matches            []GrepMatch
	TotalMatched       int
	TotalFilesSearched int
	TotalFiles         int
	FilteredFileCount  int
	NextFileOffset     int
	RegexFallbackError string
}

// FileItem represents an indexed file.
type FileItem struct {
	RelativePath       string
	FileName           string
	GitStatus          string
	Size               uint64
	Modified           uint64
	TotalFrecencyScore int64
	IsBinary           bool
}

// SearchResult holds file search / fuzzy search results.
type SearchResult struct {
	Files        []FileItem
	TotalMatched int
	TotalFiles   int
}

// Options configure the fff engine instance.
type Options struct {
	Watch                 bool
	EnableContentIndexing bool
	EnableMmapCache       bool
	AIMode                bool
	CacheBudgetMaxBytes   uint64
	CacheBudgetMaxFiles   uint64
}

// LiveGrepOptions configure a content grep operation.
type LiveGrepOptions struct {
	Mode                GrepMode
	MaxFileSize         uint64
	MaxMatchesPerFile   uint32
	SmartCase           bool
	FileOffset          uint32
	PageLimit           uint32
	TimeBudgetMs        uint64
	BeforeContext       uint32
	AfterContext        uint32
	ClassifyDefinitions bool
}

// SearchFileOptions configure a fuzzy file search operation.
type SearchFileOptions struct {
	CurrentFile          string
	MaxThreads           uint32
	PageIndex            uint32
	PageSize             uint32
	ComboBoostMultiplier int32
	MinComboCount        uint32
}

// Manager manages long-lived fff engine instances across multiple workspace roots.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	opts      Options
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// DefaultManager returns the global singleton fff manager.
func DefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManager = &Manager{
			instances: make(map[string]*Instance),
			opts: Options{
				Watch:                 true,
				EnableContentIndexing: true,
			},
		}
	})
	return defaultManager
}

// GetOrInit retrieves an existing instance for workDir or initializes a new one.
func (m *Manager) GetOrInit(workDir string) (*Instance, error) {
	if !Available() {
		return nil, nil
	}

	absDir, err := filepath.Abs(workDir)
	if err != nil {
		absDir = workDir
	}

	m.mu.RLock()
	inst, ok := m.instances[absDir]
	m.mu.RUnlock()
	if ok && inst != nil {
		return inst, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if inst, ok = m.instances[absDir]; ok && inst != nil {
		return inst, nil
	}

	inst, err = NewInstance(absDir, m.opts)
	if err != nil {
		return nil, err
	}

	// Trigger initial background scan wait with a brief timeout so it begins immediately
	go inst.WaitForScan(5 * time.Second)

	m.instances[absDir] = inst
	return inst, nil
}

// Close destroys all instances managed by this manager.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inst := range m.instances {
		if inst != nil {
			inst.Close()
		}
	}
	m.instances = make(map[string]*Instance)
}
