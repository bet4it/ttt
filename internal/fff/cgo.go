//go:build cgo && fff

package fff

/*
#include <fff.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// Available reports whether the native fff CGO engine is compiled and usable.
func Available() bool {
	return true
}

// Instance represents an active fff search engine instance bound to a workspace directory.
type Instance struct {
	mu       sync.RWMutex
	handle   unsafe.Pointer
	basePath string
	closed   bool
}

// NewInstance creates and initializes a new fff engine instance for the given base path.
func NewInstance(basePath string, opts Options) (*Instance, error) {
	cBasePath := C.CString(basePath)
	defer C.free(unsafe.Pointer(cBasePath))

	var cOpts C.struct_FffCreateOptions
	cOpts.version = C.FFF_CREATE_OPTIONS_VERSION
	cOpts.base_path = cBasePath
	cOpts.watch = C.bool(opts.Watch)
	cOpts.enable_content_indexing = C.bool(opts.EnableContentIndexing)
	cOpts.enable_mmap_cache = C.bool(opts.EnableMmapCache)
	cOpts.ai_mode = C.bool(opts.AIMode)
	cOpts.cache_budget_max_bytes = C.uint64_t(opts.CacheBudgetMaxBytes)
	cOpts.cache_budget_max_files = C.uint64_t(opts.CacheBudgetMaxFiles)

	res := C.fff_create_instance_with(&cOpts)
	if res == nil {
		return nil, errors.New("fff_create_instance_with returned null")
	}
	defer C.fff_free_result(res)

	if !bool(res.success) {
		errStr := "unknown fff initialization error"
		if res.error != nil {
			errStr = C.GoString(res.error)
		}
		return nil, fmt.Errorf("fff initialization failed: %s", errStr)
	}

	inst := &Instance{
		handle:   res.handle,
		basePath: basePath,
	}

	runtime.SetFinalizer(inst, (*Instance).Close)
	return inst, nil
}

// Close destroys the fff instance and frees associated resources.
func (inst *Instance) Close() {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.closed || inst.handle == nil {
		return
	}
	C.fff_destroy(inst.handle)
	inst.handle = nil
	inst.closed = true
}

// BasePath returns the root directory indexed by this instance.
func (inst *Instance) BasePath() string {
	return inst.basePath
}

// WaitForScan waits up to the given timeout for the initial background scan to complete.
// Returns true if the scan is complete, false on timeout.
func (inst *Instance) WaitForScan(timeout time.Duration) bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.closed || inst.handle == nil {
		return false
	}

	ms := uint64(timeout.Milliseconds())
	if ms == 0 {
		ms = 1
	}

	res := C.fff_wait_for_scan(inst.handle, C.uint64_t(ms))
	if res == nil {
		return false
	}
	defer C.fff_free_result(res)

	return bool(res.success) && res.int_value == 1
}

// LiveGrep performs content search across the indexed files.
func (inst *Instance) LiveGrep(query string, opts LiveGrepOptions) (*GrepResult, error) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.closed || inst.handle == nil {
		return nil, errors.New("fff instance is closed")
	}

	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	maxFileSize := opts.MaxFileSize
	if maxFileSize == 0 {
		maxFileSize = 10 * 1024 * 1024
	}
	pageLimit := opts.PageLimit
	if pageLimit == 0 {
		pageLimit = 100
	}
	maxMatchesPerFile := opts.MaxMatchesPerFile
	if maxMatchesPerFile == 0 {
		maxMatchesPerFile = 100
	}

	res := C.fff_live_grep(
		inst.handle,
		cQuery,
		C.uint8_t(opts.Mode),
		C.uint64_t(maxFileSize),
		C.uint32_t(maxMatchesPerFile),
		C.bool(opts.SmartCase),
		C.uint32_t(opts.FileOffset),
		C.uint32_t(pageLimit),
		C.uint64_t(opts.TimeBudgetMs),
		C.uint32_t(opts.BeforeContext),
		C.uint32_t(opts.AfterContext),
		C.bool(opts.ClassifyDefinitions),
	)
	if res == nil {
		return nil, errors.New("fff_live_grep returned null")
	}
	defer C.fff_free_result(res)

	if !bool(res.success) {
		errStr := "unknown grep error"
		if res.error != nil {
			errStr = C.GoString(res.error)
		}
		return nil, fmt.Errorf("grep failed: %s", errStr)
	}

	gr := (*C.struct_FffGrepResult)(res.handle)
	if gr == nil {
		return &GrepResult{}, nil
	}
	defer C.fff_free_grep_result(gr)

	count := int(gr.count)
	result := &GrepResult{
		TotalMatched:       int(gr.total_matched),
		TotalFilesSearched: int(gr.total_files_searched),
		TotalFiles:         int(gr.total_files),
		FilteredFileCount:  int(gr.filtered_file_count),
		NextFileOffset:     int(gr.next_file_offset),
	}
	if gr.regex_fallback_error != nil {
		result.RegexFallbackError = C.GoString(gr.regex_fallback_error)
	}

	if count > 0 && gr.items != nil {
		items := unsafe.Slice(gr.items, count)
		result.Matches = make([]GrepMatch, count)
		for i := range items {
			item := &items[i]
			m := GrepMatch{
				LineNumber: int(item.line_number),
				Col:        int(item.col),
			}
			if item.relative_path != nil {
				m.RelativePath = C.GoString(item.relative_path)
			}
			if item.file_name != nil {
				m.FileName = C.GoString(item.file_name)
			}
			if item.git_status != nil {
				m.GitStatus = C.GoString(item.git_status)
			}
			if item.line_content != nil {
				m.LineContent = C.GoString(item.line_content)
			}

			if item.match_ranges_count > 0 && item.match_ranges != nil {
				rCount := int(item.match_ranges_count)
				rSlice := unsafe.Slice(item.match_ranges, rCount)
				m.MatchRanges = make([]MatchRange, rCount)
				for rIdx := range rSlice {
					m.MatchRanges[rIdx] = MatchRange{
						Start: int(rSlice[rIdx].start),
						End:   int(rSlice[rIdx].end),
					}
				}
			}
			result.Matches[i] = m
		}
	}

	return result, nil
}

// SearchFiles performs fuzzy file path search.
func (inst *Instance) SearchFiles(query string, opts SearchFileOptions) (*SearchResult, error) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.closed || inst.handle == nil {
		return nil, errors.New("fff instance is closed")
	}

	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var cCurFile *C.char
	if opts.CurrentFile != "" {
		cCurFile = C.CString(opts.CurrentFile)
		defer C.free(unsafe.Pointer(cCurFile))
	}

	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = 100
	}
	comboBoost := opts.ComboBoostMultiplier
	if comboBoost == 0 {
		comboBoost = 100
	}
	minCombo := opts.MinComboCount
	if minCombo == 0 {
		minCombo = 3
	}

	res := C.fff_search(
		inst.handle,
		cQuery,
		cCurFile,
		C.uint32_t(opts.MaxThreads),
		C.uint32_t(opts.PageIndex),
		C.uint32_t(pageSize),
		C.int32_t(comboBoost),
		C.uint32_t(minCombo),
	)
	if res == nil {
		return nil, errors.New("fff_search returned null")
	}
	defer C.fff_free_result(res)

	if !bool(res.success) {
		errStr := "unknown search error"
		if res.error != nil {
			errStr = C.GoString(res.error)
		}
		return nil, fmt.Errorf("search failed: %s", errStr)
	}

	sr := (*C.struct_FffSearchResult)(res.handle)
	if sr == nil {
		return &SearchResult{}, nil
	}
	defer C.fff_free_search_result(sr)

	count := int(sr.count)
	result := &SearchResult{
		TotalMatched: int(sr.total_matched),
		TotalFiles:   int(sr.total_files),
	}

	if count > 0 && sr.items != nil {
		items := unsafe.Slice(sr.items, count)
		result.Files = make([]FileItem, count)
		for i := range items {
			item := &items[i]
			fi := FileItem{
				Size:               uint64(item.size),
				Modified:           uint64(item.modified),
				TotalFrecencyScore: int64(item.total_frecency_score),
				IsBinary:           bool(item.is_binary),
			}
			if item.relative_path != nil {
				fi.RelativePath = C.GoString(item.relative_path)
			}
			if item.file_name != nil {
				fi.FileName = C.GoString(item.file_name)
			}
			if item.git_status != nil {
				fi.GitStatus = C.GoString(item.git_status)
			}
			result.Files[i] = fi
		}
	}

	return result, nil
}

// ListFiles returns all indexed file relative paths using glob matching.
func (inst *Instance) ListFiles(maxCount uint32) ([]string, error) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.closed || inst.handle == nil {
		return nil, errors.New("fff instance is closed")
	}

	if maxCount == 0 {
		maxCount = 100000
	}

	cPattern := C.CString("*")
	defer C.free(unsafe.Pointer(cPattern))

	res := C.fff_glob(
		inst.handle,
		cPattern,
		nil,
		0,
		0,
		C.uint32_t(maxCount),
	)
	if res == nil {
		return nil, errors.New("fff_glob returned null")
	}
	defer C.fff_free_result(res)

	if !bool(res.success) {
		errStr := "unknown glob error"
		if res.error != nil {
			errStr = C.GoString(res.error)
		}
		return nil, fmt.Errorf("glob failed: %s", errStr)
	}

	sr := (*C.struct_FffSearchResult)(res.handle)
	if sr == nil {
		return nil, nil
	}
	defer C.fff_free_search_result(sr)

	count := int(sr.count)
	if count == 0 || sr.items == nil {
		return nil, nil
	}

	items := unsafe.Slice(sr.items, count)
	files := make([]string, count)
	for i := range items {
		if items[i].relative_path != nil {
			files[i] = C.GoString(items[i].relative_path)
		}
	}

	return files, nil
}
