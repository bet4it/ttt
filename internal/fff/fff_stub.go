//go:build !cgo || !fff

package fff

import (
	"errors"
	"time"
)

// Available reports whether the native fff engine is compiled and usable.
func Available() bool {
	return false
}

// Instance is a stub when CGO is disabled.
type Instance struct{}

func NewInstance(basePath string, opts Options) (*Instance, error) {
	return nil, errors.New("fff is not available without CGO")
}

func (inst *Instance) Close() {}

func (inst *Instance) BasePath() string { return "" }

func (inst *Instance) WaitForScan(timeout time.Duration) bool { return false }

func (inst *Instance) LiveGrep(query string, opts LiveGrepOptions) (*GrepResult, error) {
	return nil, errors.New("fff is not available without CGO")
}

func (inst *Instance) SearchFiles(query string, opts SearchFileOptions) (*SearchResult, error) {
	return nil, errors.New("fff is not available without CGO")
}

func (inst *Instance) ListFiles(maxCount uint32) ([]string, error) {
	return nil, errors.New("fff is not available without CGO")
}
