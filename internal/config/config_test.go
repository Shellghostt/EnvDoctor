package config_test

import (
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/config"
)

func TestValidateAcceptsDefaults(t *testing.T) {
	if err := (config.Options{}).Validate(); err != nil {
		t.Fatalf("empty options should be valid: %v", err)
	}
}

func TestValidateAcceptsRuleAndComponent(t *testing.T) {
	opts := config.Options{Rule: "ENV006", Component: "docker"}
	if err := opts.Validate(); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
}

func TestValidateRejectsBadRuleID(t *testing.T) {
	for _, value := range []string{"ENV1", "env001", "ENV0001", "ENV00A", "rule"} {
		opts := config.Options{Rule: value}
		if err := opts.Validate(); err == nil {
			t.Errorf("rule %q should be rejected", value)
		} else if !strings.Contains(err.Error(), "ENV001") {
			t.Errorf("error for %q should explain the expected form: %v", value, err)
		}
	}
}

func TestValidateRejectsBadComponent(t *testing.T) {
	for _, value := range []string{"Docker", "docker daemon", "-docker", "docker_daemon"} {
		opts := config.Options{Component: value}
		if err := opts.Validate(); err == nil {
			t.Errorf("component %q should be rejected", value)
		}
	}
}

func TestDefaults(t *testing.T) {
	if config.SchemaVersion != 1 {
		t.Errorf("schema version = %d", config.SchemaVersion)
	}
	if config.CollectorTimeout.Seconds() != 2 {
		t.Errorf("collector timeout = %s, want 2s", config.CollectorTimeout)
	}
}
