package collector

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/shellghostt/envdoctor/internal/system"
)

// PathCollector splits PATH and checks each entry with file metadata.
//
// It never executes an entry. A directory that contains a malicious executable
// is reported as existing, not run.
type PathCollector struct {
	Getenv     func(string) string
	Stat       func(string) (os.FileInfo, error)
	Separator  string
	Normalizer func(string) string
}

// NewPathCollector binds the collector to the running process.
func NewPathCollector() PathCollector {
	return PathCollector{
		Getenv:    os.Getenv,
		Stat:      os.Stat,
		Separator: string(os.PathListSeparator),
	}
}

// Name implements Collector.
func (c PathCollector) Name() string { return "path" }

// Collect implements Collector.
func (c PathCollector) Collect(ctx context.Context) (Part, error) {
	entries := make([]PathEntry, 0, 32)
	seen := make(map[string]bool)
	position := 0

	value := c.Getenv("PATH")
	for _, raw := range strings.Split(value, c.Separator) {
		if err := ctx.Err(); err != nil {
			return Part{Path: &PathInfo{Entries: entries}}, err
		}
		if raw == "" {
			// An empty element means "current directory" on Unix and is almost
			// never intentional. It is not a directory that can be checked, so
			// it is skipped rather than reported as existing.
			continue
		}

		target := unquote(raw)
		info, err := c.Stat(target)
		if err != nil && !system.IsMissingPath(err) {
			return Part{}, fmt.Errorf("inspect PATH entry %q: %w", raw, err)
		}
		exists := err == nil
		isDir := exists && info.IsDir()

		key := c.normalize(target)
		duplicate := seen[key]
		seen[key] = true

		entries = append(entries, PathEntry{
			Raw:       raw,
			Exists:    exists,
			IsDir:     isDir,
			Position:  position,
			Duplicate: duplicate,
		})
		position++
	}

	return Part{Path: &PathInfo{Entries: entries}}, nil
}

func (c PathCollector) normalize(value string) string {
	if c.Normalizer != nil {
		return c.Normalizer(value)
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(value)
	}
	return value
}

// unquote removes the surrounding double quotes that Windows allows in a PATH
// entry, so that `"C:\Program Files\bin"` can be checked. Raw retains the
// quotes for display.
func unquote(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value[1 : len(value)-1]
	}
	return value
}
