//go:build windows

package collector_test

import (
	"context"
	"os"
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
)

func TestMalformedWindowsPathIsReportedWithRawEntry(t *testing.T) {
	raw := ` C:\Windows\System32`
	c := collector.PathCollector{Getenv: func(string) string { return raw }, Separator: ";", Stat: os.Stat}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("malformed path degraded collection: %v", err)
	}
	if part.Path == nil || len(part.Path.Entries) != 1 || part.Path.Entries[0].Raw != raw || part.Path.Entries[0].Exists {
		t.Fatalf("malformed raw entry was not preserved as missing: %+v", part.Path)
	}
}
