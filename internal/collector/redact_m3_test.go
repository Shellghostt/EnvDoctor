package collector_test

import (
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
)

func TestRedactionCoversHomePathsInsideURLsAndCredentialURLPaths(t *testing.T) {
	for _, tc := range []struct{ value, home string }{
		{"unix:///home/fixture-user/.docker/engine.sock", "/home/fixture-user"},
		{"file:///C:/Users/fixture-user/config", `C:\Users\fixture-user`},
		{"error https://user:FAKE_SECRET@proxy.invalid/FAKE_PATH?token=FAKE_QUERY", ""},
		{"ssh://user:FAKE_SECRET@remote.invalid/home/fixture-user/socket", "/home/fixture-user"},
		{"open https:/user:FAKE_SECRET@remote.invalid/FAKE_PATH?token=FAKE_QUERY", ""},
	} {
		redacted := collector.RedactText(tc.value, tc.home)
		if strings.Contains(redacted, "fixture-user") || strings.Contains(redacted, "FAKE_") || strings.Contains(redacted, "user:") {
			t.Fatalf("URL redaction leaked input: %s", redacted)
		}
	}
}
