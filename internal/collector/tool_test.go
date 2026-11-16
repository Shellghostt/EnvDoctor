package collector_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
)

type toolExit int

func (e toolExit) Error() string { return fmt.Sprintf("exit %d", e) }
func (e toolExit) ExitCode() int { return int(e) }

func TestDockerCollectionDistinguishesAbsenceOutageAndBadMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, output               string
		installed, reachable, fail bool
		err                        error
	}{
		{name: "absent"},
		{name: "healthy", installed: true, reachable: true, output: `{"Client":{"Version":"29.9.2"},"Server":{"Version":"29.9.2"}}`},
		{name: "unreachable", installed: true, output: `{"Client":{"Version":"29.9.2"}}`, err: toolExit(1)},
		{name: "bad metadata", installed: true, fail: true, output: `{"Client":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := collector.DockerCollector{FindTool: func(string) string {
				if tc.installed {
					return "known-docker"
				}
				return ""
			}, RunTool: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if len(args) != 5 || args[0] != "--host" || !strings.Contains(args[1], "docker") {
					t.Fatalf("probe did not select the fixed local socket: %v", args)
				}
				return []byte(tc.output), tc.err
			}}
			part, err := c.Collect(context.Background())
			if tc.fail {
				if err == nil && len(part.Errors) == 0 {
					t.Fatal("invalid metadata accepted without degradation")
				}
				return
			}
			if err != nil || part.Docker == nil || part.Docker.Installed != tc.installed || part.Docker.DaemonReachable != tc.reachable {
				t.Fatalf("Docker state: %+v %v", part.Docker, err)
			}
		})
	}
}

func TestGitMissingIdentityIsDifferentFromInvalidConfiguration(t *testing.T) {
	for _, code := range []toolExit{1, 128} {
		c := collector.GitCollector{FindTool: func(string) string { return "known-git" }, RunTool: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if args[0] == "--version" {
				return []byte("git version 2.53.0"), nil
			}
			return nil, code
		}}
		part, err := c.Collect(context.Background())
		if code == 1 && (err != nil || part.Git == nil || part.Git.UserName != nil || part.Git.UserEmail != nil) {
			t.Fatalf("missing optional identity degraded Git: %+v %v", part.Git, err)
		}
		if code == 128 && !errors.Is(err, code) {
			t.Fatalf("invalid Git configuration was hidden: %v", err)
		}
	}
}
