package bundle_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

var captureTime = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

func sensitiveDocument() report.Document {
	name, email := "SyntheticUser42", "synthetic42@private.invalid"
	s := collector.Snapshot{SchemaVersion: 1, CollectedAt: captureTime,
		Platform: &collector.Platform{OS: "windows", Arch: "amd64", Version: "11"},
		Shell:    &collector.Shell{Name: "powershell", Path: `C:\Users\SyntheticUser42\shell.exe`},
		Path: &collector.PathInfo{Entries: []collector.PathEntry{
			{Raw: `C:\Users\SyntheticUser42\bin`, Exists: true, IsDir: true, Position: 0},
			{Raw: `C:\Users\SyntheticUser42\gone`, Position: 1},
		}},
		Runtimes: []collector.Runtime{
			{Name: "python", Version: "3.12.1", Manager: "system", Active: true, Path: `C:\Users\SyntheticUser42\bin\python.exe`, Resolution: "known", ResolvedPath: `C:\Users\SyntheticUser42\python.exe`, EnvironmentRoot: `C:\Users\SyntheticUser42\venv`},
			{Name: "python", Version: "3.11.9", Manager: "venv", Active: true, Path: `C:\Users\SyntheticUser42\other\python.exe`},
		},
		Git:            &collector.Git{Version: "2.53.0", UserName: &name, UserEmail: &email, IdentityChecked: true, Autocrlf: "true", AutocrlfOrigin: `file:C:\Users\SyntheticUser42\.gitconfig`},
		Docker:         &collector.Docker{Installed: true, CLIVersion: "29.9.2", Context: "local-default", ProbeStatus: "available", DaemonReachable: true, SelectedContext: "private-context42", SelectedHost: "ssh://SyntheticUser42@private-host42/secret42"},
		RequestedPorts: []int{3000}, Ports: []collector.Port{{Port: 3000, Protocol: "tcp", State: "listen", PID: 12345, Process: "private-process42"}},
		Dependencies: &collector.Dependencies{ProjectRoot: `C:\Users\SyntheticUser42\project`, RequirementsFile: `C:\Users\SyntheticUser42\project\envdoctor.requirements.json`, GitAutocrlf: "input",
			Requirements: []collector.Dependency{{Kind: "library", Name: "openssl", Status: "missing", Path: `C:\Users\SyntheticUser42\openssl`}},
			Package:      &collector.PackageEvidence{Manifest: `C:\Users\SyntheticUser42\project\package.json`, Declared: &collector.PackageManager{Name: "pnpm", Version: "9.0.0"}, Runner: &collector.PackageManager{Name: "npm", Version: "10.0.0"}, Lockfiles: []string{"pnpm-lock.yaml"}}},
		PythonEnvironment: &collector.PythonEnvironment{Root: `C:\Users\SyntheticUser42\venv`, CanonicalRoot: `c:\users\syntheticuser42\venv`, MetadataKnown: true},
		Proxies:           []collector.Proxy{{Variable: "HTTPS_PROXY", URL: "http://encoded%2Dsecret42:p%40ss@private-host42:8080/path42?token=secret42", Host: "private-host42", Port: 8080, Probe: "unreachable"}},
		Errors:            []collector.CollectorError{{Collector: "shell", Message: "failed reading C:\\Users\\SyntheticUser42\\shell; token=secret42"}},
	}
	findings, _ := diagnostic.DefaultEngine().Evaluate(s, diagnostic.Filter{})
	d := report.NewDocument("0.1.0", captureTime, s, findings, nil, 12)
	// Original finding text is deliberately unsafe. Sharing must recompute it.
	d.Findings = append(d.Findings, diagnostic.Finding{RuleID: "ENV999", Title: "secret42", Detected: []string{"secret42"}, Severity: diagnostic.SeverityCritical})
	return d
}

func members(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, f := range r.File {
		stream, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		_, err = b.ReadFrom(stream)
		closeErr := stream.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("member read: %v / %v", err, closeErr)
		}
		result[f.Name] = b.Bytes()
	}
	return result
}

func TestM4BundleSharingAllowlistAndPresence(t *testing.T) {
	original := sensitiveDocument()
	data, err := bundle.Build(context.Background(), original, bundle.Options{IncludeLogs: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= 2_000_000 {
		t.Fatalf("archive is %d bytes", len(data))
	}
	all := members(t, data)
	if len(all) != 6 {
		t.Fatalf("members=%d", len(all))
	}
	for name, body := range all {
		for _, secret := range []string{"SyntheticUser42", "syntheticuser42", "synthetic42@private.invalid", "secret42", "private-host42", "private-context42", "private-process42", "encoded%2Dsecret42", "12345"} {
			if strings.Contains(string(body), secret) || strings.Contains(name, secret) {
				t.Fatalf("%s leaks %q", name, secret)
			}
		}
	}
	if !bytes.Contains(all["snapshot.json"], []byte(`"userNamePresent":true`)) || bytes.Contains(all["snapshot.json"], []byte(`"userName":`)) {
		t.Fatal("Git identity was not represented solely by presence")
	}
	loaded, err := bundle.Read(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Snapshot.Git.UserName == nil || *loaded.Snapshot.Git.UserName == "SyntheticUser42" {
		t.Fatal("Git presence lost or identity retained")
	}
	if loaded.Source.CapturedAt != captureTime || loaded.Source.CaptureTool.Version != "0.1.0" || len(loaded.Source.Omissions) == 0 {
		t.Fatal("capture provenance or omissions lost")
	}
	results, err := diagnostic.DefaultEngine().Evaluate(loaded.Snapshot, diagnostic.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, f := range results {
		ids[f.RuleID] = true
	}
	for _, id := range []string{"ENV001", "ENV003", "ENV005", "ENV008", "ENV009", "ENV010", "ENV012"} {
		if !ids[id] {
			t.Errorf("retained diagnostic evidence lost %s", id)
		}
	}
	if ids["ENV011"] || ids["ENV999"] {
		t.Fatal("normalization or saved verdict changed evaluation")
	}
	before, _ := json.Marshal(original)
	after, _ := json.Marshal(sensitiveDocument())
	if !bytes.Equal(before, after) {
		t.Fatal("source document mutated")
	}
}

func TestM4UnsupportedSharingValuesAreInconclusive(t *testing.T) {
	doc := sensitiveDocument()
	doc.Snapshot.Runtimes[0].Version = "token=SECRET-UNSUPPORTED"
	data, err := bundle.Build(context.Background(), doc, bundle.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range members(t, data) {
		if bytes.Contains(body, []byte("SECRET-UNSUPPORTED")) {
			t.Fatal("unsupported version leaked")
		}
	}
	loaded, err := bundle.Read(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := diagnostic.DefaultEngine().Evaluate(loaded.Snapshot, diagnostic.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if finding.RuleID == "ENV001" {
			t.Fatal("omitted runtime version treated as conclusive")
		}
	}
	degraded := false
	for _, failure := range loaded.Snapshot.Errors {
		degraded = degraded || failure.Collector == "runtimes"
	}
	if !degraded {
		t.Fatal("required omitted evidence is not degraded")
	}
}
