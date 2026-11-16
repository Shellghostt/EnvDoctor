package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/shellghostt/envdoctor/internal/cli"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

type neverCollect struct{}

func (neverCollect) Name() string { return "forbidden" }
func (neverCollect) Collect(context.Context) (collector.Part, error) {
	panic("offline command invoked a collector")
}

func TestMetadataCommandsAreOfflineAndMatchRegistryAndPages(t *testing.T) {
	var out, errs bytes.Buffer
	app := newApp(&out, &errs)
	app.Collectors = []collector.Collector{neverCollect{}}
	if code := cli.Main([]string{"envdoctor", "rules", "--json"}, app); code != 0 || errs.Len() != 0 {
		t.Fatalf("rules exit=%d stderr=%s", code, errs.String())
	}
	var registry diagnostic.RegistryDocument
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := decoder.Decode(&registry); err != nil {
		t.Fatal(err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		t.Fatal("rules emitted multiple JSON documents")
	}
	if registry.SchemaVersion != 1 || !reflect.DeepEqual(registry.Rules, diagnostic.BuiltinMetadata()) {
		t.Fatal("rules JSON diverges from the registry")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "rules-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	id := "https://github.com/shellghostt/envdoctor/schema/rules-v1.schema.json"
	if err := compiler.AddResource(id, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainstSchema(t, schema, out.Bytes())
	for _, rule := range registry.Rules {
		out.Reset()
		errs.Reset()
		if code := cli.Main([]string{"envdoctor", "explain", rule.RuleID}, app); code != 0 || errs.Len() != 0 {
			t.Fatalf("explain %s exit=%d stderr=%s", rule.RuleID, code, errs.String())
		}
		page, err := os.ReadFile(filepath.Join("..", "..", "docs", "rules", rule.RuleID+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if out.String() != string(page) || out.String() != diagnostic.RuleMarkdown(rule) || strings.Contains(out.String(), "\x1b") {
			t.Fatalf("explain/page mismatch for %s", rule.RuleID)
		}
	}
	for _, args := range [][]string{{"explain", "ENV013"}, {"explain"}, {"explain", "ENV006", "ENV007"}, {"rules", "extra"}, {"rules", "--probe-network"}, {"diagnose", "--fixture-dir", "unused", "--probe-network"}, {"diagnose", "--fixture-dir", "unused", "--package-manager", "npm"}, {"diagnose", "--package-manager", "npm@^10"}} {
		out.Reset()
		errs.Reset()
		if code := cli.Main(append([]string{"envdoctor"}, args...), app); code != cli.ExitUsage || out.Len() != 0 {
			t.Fatalf("invalid metadata/option usage: %v code=%d out=%s err=%s", args, code, out.String(), errs.String())
		}
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("output failed") }

func TestMetadataWriteErrorsUseToolFailureExit(t *testing.T) {
	for _, args := range [][]string{{"rules"}, {"rules", "--json"}, {"explain", "ENV006"}} {
		var errs bytes.Buffer
		app := newApp(nil, &errs)
		app.Stdout = failedWriter{}
		app.Collectors = []collector.Collector{neverCollect{}}
		if code := cli.Main(append([]string{"envdoctor"}, args...), app); code != cli.ExitToolFailure {
			t.Fatalf("write failure: %v exit=%d", args, code)
		}
	}
}

type partialProxy struct{ part collector.Part }

func (partialProxy) Name() string                                      { return "proxy" }
func (p partialProxy) Collect(context.Context) (collector.Part, error) { return p.part, nil }

func TestNewEvidenceAndNoticesAreRedactedEvenWithoutHomeDiscovery(t *testing.T) {
	for _, home := range []string{`C:\Users\fixture-user`, "/home/fixture-user", "/Users/fixture-user"} {
		var out, errs bytes.Buffer
		app := newApp(&out, &errs)
		app.Engine = nil
		app.HomeDir = func() (string, error) { return home, nil }
		app.Collectors = []collector.Collector{partialProxy{part: collector.Part{
			Proxies:      []collector.Proxy{{Variable: "HTTPS_PROXY", URL: "https://user:FAKE_SECRET@proxy.invalid:443/private?token=FAKE_QUERY", Host: "proxy.invalid", Port: 443, Probe: "unknown"}},
			Dependencies: &collector.Dependencies{ProjectRoot: home + "/project", RequirementsFile: home + "/project/envdoctor.requirements.json"},
			Git:          &collector.Git{Version: "2.45.0", UserNameOrigin: "file:" + home + "/.gitconfig"},
			Errors:       []collector.CollectorError{{Collector: "proxy", Message: "denied at " + home + "/config https://user:FAKE_SECRET@proxy.invalid/path?token=FAKE_QUERY"}},
		}}}
		code := cli.Main([]string{"envdoctor", "diagnose", "--json", "--verbose"}, app)
		if code != cli.ExitToolFailure {
			t.Fatalf("partial failure exit=%d", code)
		}
		validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
		joined := out.String() + errs.String()
		for _, value := range []string{"FAKE_SECRET", "FAKE_QUERY", strings.ReplaceAll(home, `\`, `\\`), home} {
			if strings.Contains(joined, value) {
				t.Fatalf("new evidence leaked %s: %s", value, joined)
			}
		}
		var document report.Document
		if err := json.Unmarshal(out.Bytes(), &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Findings) != 0 || len(document.Snapshot.Proxies) != 1 || document.Snapshot.Dependencies == nil || len(document.Collectors) != 1 || document.Collectors[0].Error == "" {
			t.Fatalf("partial evidence/degradation not retained: %+v", document)
		}
	}
	var out, errs bytes.Buffer
	app := newApp(&out, &errs)
	app.HomeDir = func() (string, error) { return "", os.ErrPermission }
	app.Collectors = []collector.Collector{staticCollector{name: "proxy", err: errors.New("failed https://user:FAKE_SECRET@host/?token=FAKE_QUERY")}}
	if code := cli.Main([]string{"envdoctor", "diagnose", "--json", "--verbose"}, app); code != 3 {
		t.Fatalf("failure exit=%d", code)
	}
	if strings.Contains(out.String()+errs.String(), "FAKE_") {
		t.Fatal("failed home discovery disabled credential redaction")
	}
	code := cli.Main([]string{"envdoctor", "explain", "https://user:FAKE_SECRET@host/?token=FAKE_QUERY"}, app)
	if code != 4 || strings.Contains(errs.String(), "FAKE_") {
		t.Fatal("usage error leaked URL credentials")
	}
}

func TestNormalDiagnosisDoesNotConnectToRemoteOverrides(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	endpoint := "http://" + listener.Addr().String()
	t.Setenv("HTTP_PROXY", endpoint)
	t.Setenv("HTTPS_PROXY", endpoint)
	t.Setenv("ALL_PROXY", endpoint)
	t.Setenv("http_proxy", endpoint)
	t.Setenv("https_proxy", endpoint)
	t.Setenv("all_proxy", endpoint)
	t.Setenv("DOCKER_HOST", "tcp://"+listener.Addr().String())
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_CONFIG", "")
	var out, errs bytes.Buffer
	app := newApp(&out, &errs)
	app.Collectors = nil
	app.Engine = nil
	start := time.Now()
	code := cli.Main([]string{"envdoctor", "diagnose", "--json"}, app)
	if code == 4 || out.Len() == 0 || time.Since(start) > 4*time.Second {
		t.Fatalf("normal diagnosis: exit=%d elapsed=%s err=%s", code, time.Since(start), errs.String())
	}
	var document report.Document
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	for _, p := range document.Snapshot.Proxies {
		if p.Probe != "untested" {
			t.Fatalf("default diagnosis probed proxy: %+v", p)
		}
	}
	if err := listener.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err == nil {
		_ = conn.Close()
		t.Fatal("normal diagnosis connected to a remote override")
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("listener failure: %v", err)
	}
	validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
}
