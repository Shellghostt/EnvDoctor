package bundle_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

func TestM4All172GoldenCapturesRoundTripAcrossOS(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "golden")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 172 {
		t.Fatalf("original golden corpus changed: %d", len(files))
	}
	osSeen := map[string]bool{}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			fixture, err := diagnostic.LoadFixture(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			s := fixture.Snapshot
			if s.Platform != nil {
				osSeen[s.Platform.OS] = true
			}
			findings, err := diagnostic.DefaultEngine().Evaluate(s, diagnostic.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			doc := report.NewDocument("0.1.0", s.CollectedAt, s, findings, nil, 12)
			data, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
			if err != nil {
				t.Fatal(err)
			}
			capture, err := bundle.Read(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := diagnostic.DefaultEngine().Evaluate(capture.Snapshot, diagnostic.Filter{RuleID: fixture.RuleID})
			if err != nil {
				t.Fatal(err)
			}
			metadata, _ := diagnostic.LookupRule(fixture.RuleID)
			blocked := false
			for _, failure := range capture.Snapshot.Errors {
				for _, pre := range metadata.Prerequisites {
					blocked = blocked || failure.Collector == pre
				}
			}
			if blocked {
				if len(got) != 0 {
					t.Fatal("unavailable prerequisite produced conclusive findings")
				}
				return
			}
			if len(got) != len(fixture.ExpectedFindings) {
				t.Fatalf("findings lost: got=%v expected=%v", got, fixture.ExpectedFindings)
			}
			for i, f := range got {
				want := fixture.ExpectedFindings[i]
				// Evidence locations/owners intentionally change; verdict and all
				// registry-owned policy/advice must remain identical.
				f.Detected = nil
				want.Detected = nil
				if !reflect.DeepEqual(f, want) {
					t.Fatalf("retained verdict changed: got=%+v want=%+v", f, want)
				}
			}
			again, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
			if err != nil || !bytes.Equal(data, again) {
				t.Fatal("same capture did not produce deterministic aliases/archive")
			}
		})
	}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		if !osSeen[goos] {
			t.Errorf("capture OS %s missing", goos)
		}
	}
}

func TestM4EnvironmentAliasUsesCapturedOSAndExactPaths(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		doc := sensitiveDocument()
		doc.Snapshot.Errors = nil
		doc.Snapshot.Platform.OS = goos
		doc.Snapshot.Runtimes = doc.Snapshot.Runtimes[:1]
		doc.Snapshot.PythonEnvironment.CanonicalRoot = "/Capture/ENV"
		doc.Snapshot.PythonEnvironment.Root = "/Capture/ENV"
		doc.Snapshot.Runtimes[0].EnvironmentRoot = "/capture/env/./"
		data, err := bundle.Build(context.Background(), doc, bundle.Options{})
		if err != nil {
			t.Fatal(err)
		}
		capture, err := bundle.Read(context.Background(), data)
		if err != nil {
			t.Fatal(err)
		}
		results, err := diagnostic.DefaultEngine().Evaluate(capture.Snapshot, diagnostic.Filter{RuleID: "ENV011"})
		if err != nil {
			t.Fatal(err)
		}
		if (len(results) > 0) != (goos != "windows") {
			t.Fatalf("capture %s used receiving OS normalization", goos)
		}
	}
	doc := sensitiveDocument()
	doc.Snapshot.Errors = nil
	doc.Snapshot.Runtimes[1].Path = strings.ToLower(doc.Snapshot.Runtimes[0].Path)
	data, err := bundle.Build(context.Background(), doc, bundle.Options{})
	if err != nil {
		t.Fatal(err)
	}
	capture, err := bundle.Read(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if capture.Snapshot.Runtimes[0].Path == capture.Snapshot.Runtimes[1].Path {
		t.Fatal("exact path comparison collapsed distinct case")
	}
}

func TestM4RepeatedBundleLifecycleReleasesResources(t *testing.T) {
	doc := sensitiveDocument()
	data, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	beforeG := runtime.NumGoroutine()
	start := time.Now()
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := bundle.Build(ctx, doc, bundle.Options{IncludeLogs: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bundle.Read(ctx, out); err != nil {
			t.Fatal(err)
		}
		cancel()
		if _, err := bundle.Read(ctx, out); err == nil {
			t.Fatal("canceled read succeeded")
		}
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	afterG := runtime.NumGoroutine()
	t.Logf("100 bundle build/read cycles: %s; archive %d bytes; goroutines %d -> %d; retained heap %d -> %d", time.Since(start), len(data), beforeG, afterG, before.HeapAlloc, after.HeapAlloc)
	if afterG > beforeG+2 || int64(after.HeapAlloc)-int64(before.HeapAlloc) > 2<<20 {
		t.Fatal("bundle resources retained")
	}
}

func BenchmarkM4BundleBuildRead(b *testing.B) {
	doc := sensitiveDocument()
	data, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := bundle.Read(context.Background(), data); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzM4BundleReader(f *testing.F) {
	data, err := bundle.Build(context.Background(), sensitiveDocument(), bundle.Options{IncludeLogs: true})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte("not a zip"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		capture, err := bundle.Read(ctx, data)
		if err == nil {
			if _, err := json.Marshal(capture); err != nil {
				t.Fatal(err)
			}
		}
	})
}
