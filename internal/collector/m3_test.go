package collector_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/system"
)

func putFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestProxyCollectionIsOfflineRedactedAndBounded(t *testing.T) {
	values := map[string]string{"HTTP_PROXY": "http://user:secret@PROXY.invalid:8080/private?token=secret", "HTTPS_PROXY": "http://other:secret@proxy.invalid:8080", "ALL_PROXY": "socks5://proxy.invalid:1080", "NO_PROXY": "private.internal"}
	getenv := func(k string) string { return values[k] }
	calls := 0
	c := collector.ProxyCollector{Getenv: getenv, Probe: func(_ context.Context, host string, port int) (string, error) {
		calls++
		if host != "proxy.invalid" || port != 8080 && port != 1080 {
			t.Fatalf("probe contacted an unexpected target: %s:%d", host, port)
		}
		return "reachable", nil
	}}
	part, err := c.Collect(context.Background())
	if err != nil || calls != 0 || len(part.Proxies) != 3 || !part.NoProxyConfigured {
		t.Fatalf("default proxy collection: %+v calls=%d err=%v", part, calls, err)
	}
	raw, _ := json.Marshal(part.Proxies)
	for _, forbidden := range []string{"secret", "user", "private", "token"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("proxy credentials/path leaked: %s", raw)
		}
	}
	for _, p := range part.Proxies {
		if p.Probe != "untested" {
			t.Fatal("offline collection invented probe evidence")
		}
	}
	c.ProbeNetwork = true
	part, err = c.Collect(context.Background())
	if err != nil || calls != 2 || len(part.Errors) != 0 {
		t.Fatalf("duplicate endpoints were not bounded: calls=%d part=%+v err=%v", calls, part, err)
	}
	for _, p := range part.Proxies {
		if p.Probe != "reachable" {
			t.Fatalf("probe evidence: %+v", p)
		}
	}
}

func TestProxyFailuresStayUnknownAndCancellationStopsFurtherDials(t *testing.T) {
	for _, failure := range []error{os.ErrPermission, context.DeadlineExceeded, errors.New("DNS failed with https://user:FAKE_TOKEN@host/path")} {
		c := collector.ProxyCollector{ProbeNetwork: true, Getenv: func(k string) string {
			if k == "HTTPS_PROXY" {
				return "https://proxy.invalid"
			}
			return ""
		}, Probe: func(context.Context, string, int) (string, error) { return "unreachable", failure }}
		part, err := c.Collect(context.Background())
		if err != nil || len(part.Errors) != 1 || part.Proxies[0].Probe != "unknown" || strings.Contains(part.Errors[0].Message, "FAKE_TOKEN") {
			t.Fatalf("failed probe became conclusive: %+v %v", part, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	calls := 0
	c := collector.ProxyCollector{ProbeNetwork: true, Getenv: func(k string) string {
		if strings.HasSuffix(k, "PROXY") {
			return "http://" + strings.ToLower(k) + ".invalid:8080"
		}
		return ""
	}, Probe: func(ctx context.Context, _ string, _ int) (string, error) {
		calls++
		<-ctx.Done()
		return "unknown", ctx.Err()
	}}
	start := time.Now()
	_, _ = c.Collect(ctx)
	if calls != 1 || time.Since(start) > time.Second {
		t.Fatalf("cancellation did not stop probes: calls=%d elapsed=%s", calls, time.Since(start))
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err := c.Collect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled collector: %v", err)
	}
}

func TestProxyValidationAndEnvironmentPrecedence(t *testing.T) {
	for _, value := range []string{"proxy.invalid", "http://", "http://proxy.invalid:0", "http://proxy.invalid:65536", "http://proxy.invalid:", "ftp://user:FAKE_SECRET@proxy.invalid", "http://proxy.invalid:%zz", "http://::1", "http://[fe80::1%25zone]"} {
		calls := 0
		c := collector.ProxyCollector{ProbeNetwork: true, Getenv: func(k string) string {
			if k == "HTTP_PROXY" {
				return value
			}
			return ""
		}, Probe: func(context.Context, string, int) (string, error) { calls++; return "reachable", nil }}
		part, err := c.Collect(context.Background())
		if err != nil || calls != 0 || len(part.Errors) != 1 || part.Proxies[0].Probe != "invalid" || part.Proxies[0].URL != "[invalid]" {
			t.Fatalf("invalid URL probed: %q %+v %v", value, part, err)
		}
	}
	c := collector.ProxyCollector{Getenv: func(k string) string {
		switch k {
		case "HTTP_PROXY":
			return "http://upper.invalid"
		case "http_proxy":
			return "http://lower.invalid"
		case "https_proxy":
			return "https://[::1]"
		}
		return ""
	}}
	part, err := c.Collect(context.Background())
	if err != nil || part.Proxies[0].Host != "upper.invalid" || part.Proxies[1].URL != "https://[::1]:443" {
		t.Fatalf("proxy precedence/default ports: %+v %v", part, err)
	}
}

func TestDockerLocalProbeKeepsRemoteContextAsMetadata(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			root := t.TempDir()
			putFile(t, filepath.Join(root, ".docker", "config.json"), `{"currentContext":"remote","auths":{"ignored":{"auth":"FAKE_TOKEN"}}}`)
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte("remote")))
			putFile(t, filepath.Join(root, ".docker", "contexts", "meta", digest, "meta.json"), `{"Name":"remote","Endpoints":{"docker":{"Host":"tcp://user:FAKE_TOKEN@remote.invalid:2376"}}}`)
			calls := 0
			c := collector.DockerCollector{GOOS: goos, Getenv: func(k string) string {
				if k == "DOCKER_API_VERSION" {
					return "1.53"
				}
				return ""
			}, HomeDir: func() (string, error) { return root, nil }, FindTool: func(string) string { return "trusted-docker" }, RunTool: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				calls++
				host := "unix:///var/run/docker.sock"
				if goos == "windows" {
					host = "npipe:////./pipe/docker_engine"
				}
				if !reflect.DeepEqual(args, []string{"--host", host, "version", "--format", "{{json .}}"}) {
					t.Fatalf("context selected a remote probe: %v", args)
				}
				return []byte(`{"Client":{"Version":"30.0.0","ApiVersion":"1.48","DefaultAPIVersion":"1.53"},"Server":{"Version":"26.0.0","ApiVersion":"1.48","MinAPIVersion":"1.44"}}`), nil
			}}
			part, err := c.Collect(context.Background())
			if err != nil || len(part.Errors) > 0 || calls != 1 || part.Docker.SelectedContext != "remote" || part.Docker.ServerMinAPI != "1.44" || part.Docker.APIOverride != "1.53" {
				t.Fatalf("Docker evidence: %+v %v", part, err)
			}
			snapshot := collector.Snapshot{Docker: part.Docker}
			snapshot.Redact(root)
			if strings.Contains(snapshot.Docker.SelectedHost, "FAKE_TOKEN") {
				t.Fatal("Docker endpoint credentials leaked")
			}
		})
	}
}

func TestDockerHostOverrideAndInspectionStates(t *testing.T) {
	for _, tc := range []struct {
		detail, status string
		degraded       bool
	}{
		{"Cannot connect to the Docker daemon. Is the docker daemon running?", "unavailable", false},
		{"permission denied connecting to local engine", "denied", true},
		{"client version 1.20 is too old. Minimum supported API version is 1.44", "unsupported", true},
		{"unexpected internal error https://user:FAKE_TOKEN@host/path", "unknown", true},
	} {
		t.Run(tc.status, func(t *testing.T) {
			c := collector.DockerCollector{Getenv: func(k string) string {
				if k == "DOCKER_HOST" {
					return "tcp://remote.invalid:2375"
				}
				return ""
			}, FindTool: func(string) string { return "trusted" }, RunTool: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if strings.Contains(strings.Join(args, " "), "remote.invalid") {
					t.Fatal("host override reached probe")
				}
				return []byte(`{"Client":{"Version":"26.0.0","ApiVersion":"1.45"}}`), &system.ToolError{Err: toolExit(1), Detail: tc.detail}
			}}
			part, err := c.Collect(context.Background())
			if err != nil || part.Docker.ProbeStatus != tc.status || (len(part.Errors) > 0) != tc.degraded {
				t.Fatalf("Docker state: %+v %v", part, err)
			}
			for _, e := range part.Errors {
				if strings.Contains(e.Message, "FAKE_TOKEN") {
					t.Fatal("raw subprocess stderr was published")
				}
			}
		})
	}
}

func TestGitEffectiveValuesIncludeConfigOrigins(t *testing.T) {
	c := collector.GitCollector{FindTool: func(string) string { return "trusted-git" }, RunTool: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("git version 2.45.0\n"), nil
		}
		if !strings.Contains(strings.Join(args, " "), "--null --show-origin --get") {
			t.Fatalf("unbounded/unattributed Git invocation: %v", args)
		}
		key := args[len(args)-1]
		value := map[string]string{"user.name": "Developer", "user.email": "dev@example.invalid", "core.autocrlf": "input"}[key]
		return []byte("file:/home/developer/.gitconfig\x00" + value + "\x00"), nil
	}}
	part, err := c.Collect(context.Background())
	if err != nil || !part.Git.IdentityChecked || *part.Git.UserName != "Developer" || part.Git.UserEmailOrigin != "file:/home/developer/.gitconfig" || part.Git.Autocrlf != "input" {
		t.Fatalf("Git metadata: %+v %v", part, err)
	}
}

func TestGitBooleanAliasesUsePortableTypedConfigQuery(t *testing.T) {
	c := collector.GitCollector{FindTool: func(string) string { return "trusted-git" }, RunTool: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("git version 2.39.0"), nil
		}
		if strings.Contains(strings.Join(args, " "), "bool-or-str") {
			return nil, toolExit(128)
		}
		key := args[len(args)-1]
		if key != "core.autocrlf" {
			return nil, toolExit(1)
		}
		value := "yes"
		if strings.Contains(strings.Join(args, " "), "--type=bool") {
			value = "true"
		}
		return []byte("file:/fixture/config\x00" + value + "\x00"), nil
	}}
	part, err := c.Collect(context.Background())
	if err != nil || part.Git.Autocrlf != "true" || part.Git.AutocrlfOrigin != "file:/fixture/config" {
		t.Fatalf("portable Git boolean inspection: %+v %v", part, err)
	}
}

func TestDependenciesRequireExplicitManifestAndReliableAbsence(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		if k == "PATH" {
			return bin
		}
		return ""
	}
	c := collector.DependencyCollector{WorkingDir: func() (string, error) { return root, nil }, Getenv: getenv}
	putFile(t, filepath.Join(root, "main.c"), "int main() { return 0; }")
	part, err := c.Collect(context.Background())
	if err != nil || len(part.Dependencies.Requirements) != 0 {
		t.Fatalf("language file manufactured dependencies: %+v %v", part, err)
	}
	putFile(t, filepath.Join(root, collector.RequirementsFile), `{"schemaVersion":1,"requirements":[{"kind":"tool","name":"cmake"}]}`)
	part, err = c.Collect(context.Background())
	if err != nil || part.Dependencies.Requirements[0].Status != "missing" {
		t.Fatalf("explicit absence: %+v %v", part, err)
	}
	name := "cmake"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	putFile(t, filepath.Join(bin, name), "this file must never execute")
	part, err = c.Collect(context.Background())
	if err != nil || part.Dependencies.Requirements[0].Status != "present" {
		t.Fatalf("metadata presence: %+v %v", part, err)
	}
	c.Stat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	part, err = c.Collect(context.Background())
	if err != nil || len(part.Errors) == 0 || part.Dependencies.Requirements[0].Status != "unknown" {
		t.Fatalf("denial became absence: %+v %v", part, err)
	}
	c.Stat = nil
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	c.Getenv = func(k string) string {
		if k == "PATH" {
			return filepath.Join(root, "absent-bin") + sep
		}
		return ""
	}
	part, err = c.Collect(context.Background())
	if err != nil || part.Dependencies.Requirements[0].Status != "unknown" {
		t.Fatalf("empty PATH component became absence: %+v %v", part, err)
	}
	c.Getenv = func(k string) string {
		if k == "PATH" {
			return "."
		}
		return ""
	}
	part, err = c.Collect(context.Background())
	if err != nil || part.Dependencies.Requirements[0].Status != "unknown" {
		t.Fatalf("relative PATH became absence: %+v %v", part, err)
	}
}

func TestOpenSSLDeclaredPrefixNeedsHeaderAndBothLibraries(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			root := t.TempDir()
			prefix := filepath.Join(root, "openssl")
			putFile(t, filepath.Join(root, collector.RequirementsFile), `{"schemaVersion":1,"requirements":[{"kind":"library","name":"openssl","path":"openssl"}]}`)
			c := collector.DependencyCollector{GOOS: goos, Arch: "amd64", WorkingDir: func() (string, error) { return root, nil }, Getenv: func(string) string { return "" }}
			part, err := c.Collect(context.Background())
			if err != nil || part.Dependencies.Requirements[0].Status != "missing" {
				t.Fatalf("absent declared header: %+v %v", part, err)
			}
			putFile(t, filepath.Join(prefix, "include", "openssl", "ssl.h"), "header")
			ext := ".so"
			if goos == "windows" {
				ext = ".lib"
			}
			if goos == "darwin" {
				ext = ".dylib"
			}
			putFile(t, filepath.Join(prefix, "lib", "libssl"+ext), "library")
			part, err = c.Collect(context.Background())
			if err != nil || part.Dependencies.Requirements[0].Status != "missing" {
				t.Fatalf("missing crypto was ignored: %+v %v", part, err)
			}
			putFile(t, filepath.Join(prefix, "lib", "libcrypto"+ext), "library")
			part, err = c.Collect(context.Background())
			if err != nil || part.Dependencies.Requirements[0].Status != "present" {
				t.Fatalf("complete development prefix: %+v %v", part, err)
			}
			c.Stat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
			part, err = c.Collect(context.Background())
			if err != nil || len(part.Errors) == 0 || part.Dependencies.Requirements[0].Status != "unknown" {
				t.Fatalf("denied prefix: %+v %v", part, err)
			}
		})
	}
}

func TestProjectParsingIsBoundedStrictAndNeverRunsScripts(t *testing.T) {
	root := t.TempDir()
	c := collector.DependencyCollector{WorkingDir: func() (string, error) { return root, nil }, Getenv: func(string) string { return "" }}
	for _, body := range []string{`{"schemaVersion":2}`, `{"schemaVersion":1,"surprise":true}`, `{"schemaVersion":1} {}`, `{"schemaVersion":1,"requirements":[{"kind":"tool","name":"gcc","execute":"bad"}]}`, `{"schemaVersion":1,"gitAutocrlf":"maybe"}`, strings.Repeat(" ", 1<<20+1)} {
		putFile(t, filepath.Join(root, collector.RequirementsFile), body)
		if _, err := c.Collect(context.Background()); err == nil {
			t.Fatalf("invalid/bounded project metadata accepted: %.80s", body)
		}
	}
	if err := os.Remove(filepath.Join(root, collector.RequirementsFile)); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(root, "package.json"), `{"packageManager":"pnpm@9.0.0+sha512.integrity","scripts":{"install":"must never execute"}}`)
	putFile(t, filepath.Join(root, "pnpm-lock.yaml"), "data only")
	c.PackageManager = "npm@10.0.0"
	part, err := c.Collect(context.Background())
	if err != nil || part.Dependencies.Package.Declared.Version != "9.0.0" || part.Dependencies.Package.Runner.Name != "npm" || len(part.Dependencies.Package.Lockfiles) != 1 {
		t.Fatalf("package evidence: %+v %v", part, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Collect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("project collection ignored cancellation: %v", err)
	}
}

func TestUnrecognizedPythonTextShimIsInconclusive(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	env := filepath.Join(root, "venv")
	name := "python"
	sep := ":"
	if runtime.GOOS == "windows" {
		name += ".exe"
		sep = ";"
	}
	putFile(t, filepath.Join(bin, name), "#!/bin/sh\necho never-run\n")
	putFile(t, filepath.Join(env, "pyvenv.cfg"), "version = 3.12.0\n")
	c := collector.RuntimeCollector{GOOS: runtime.GOOS, Getenv: func(k string) string {
		switch k {
		case "PATH":
			return bin + sep
		case "VIRTUAL_ENV":
			return env
		}
		return ""
	}}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if part.PythonEnvironment == nil || !part.PythonEnvironment.MetadataKnown {
		t.Fatal("explicit readable environment was not recorded")
	}
	for _, r := range part.Runtimes {
		if r.Active && r.Resolution != "unknown" {
			t.Fatalf("unknown shim became an interpreter: %+v", r)
		}
	}
}
