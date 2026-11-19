package bundle_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/bundle"
)

// These archive cases were written before the archive reader/writer. Repacking
// updates digests so JSON/semantic rejection is tested independently of CRCs.
func repack(t *testing.T, all map[string][]byte, method uint16) []byte {
	t.Helper()
	var m bundle.Manifest
	if err := json.Unmarshal(all["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	m.Members = nil
	for _, name := range []string{"snapshot.json", "report.json", "report.md", "versions.json", "diagnostics.json"} {
		if body, ok := all[name]; ok {
			digest := sha256.Sum256(body)
			m.Members = append(m.Members, bundle.Member{Name: name, Size: len(body), SHA256: hex.EncodeToString(digest[:])})
		}
	}
	var err error
	all["manifest.json"], err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, name := range []string{"manifest.json", "snapshot.json", "report.json", "report.md", "versions.json", "diagnostics.json"} {
		body, ok := all[name]
		if !ok {
			continue
		}
		h := &zip.FileHeader{Name: name, Method: method}
		h.SetMode(0600)
		stream, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stream.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func validArchive(t *testing.T) []byte {
	t.Helper()
	data, err := bundle.Build(context.Background(), sensitiveDocument(), bundle.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func central(data []byte) int {
	return int(binary.LittleEndian.Uint32(data[len(data)-22+16:]))
}

func TestM4ArchiveHeaderAbuse(t *testing.T) {
	base := validArchive(t)
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"trailing-data", func(b []byte) []byte { return append(b, 0) }},
		{"prefix-data", func(b []byte) []byte { return append([]byte{0}, b...) }},
		{"multidisk", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[len(b)-22+4:], 1); return b }},
		{"zip64-count", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[len(b)-22+10:], 65535); return b }},
		{"encrypted", func(b []byte) []byte {
			binary.LittleEndian.PutUint16(b[6:], 1)
			binary.LittleEndian.PutUint16(b[central(b)+8:], 1)
			return b
		}},
		{"unsupported-method", func(b []byte) []byte {
			binary.LittleEndian.PutUint16(b[8:], 99)
			binary.LittleEndian.PutUint16(b[central(b)+10:], 99)
			return b
		}},
		{"local-central-name", func(b []byte) []byte { b[30] = 'X'; return b }},
		{"local-central-flags", func(b []byte) []byte { b[6] ^= 8; return b }},
		{"overlap", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[central(b)+42:], 1); return b }},
		{"central-extra", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[central(b)+30:], 1); return b }},
		{"local-extra", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[28:], 1); return b }},
		{"comment", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[len(b)-22+20:], 1); return b }},
		{"oversized-declared-member", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[central(b)+24:], 9<<20); return b }},
		{"crc", func(b []byte) []byte { b[central(b)+16] ^= 1; return b }},
		{"corrupt-data", func(b []byte) []byte { b[30+len("manifest.json")+3] ^= 0xff; return b }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.mutate(bytes.Clone(base))
			if _, err := bundle.Read(context.Background(), data); err == nil {
				t.Fatal("hostile archive accepted")
			}
		})
	}
}

func TestM4ArchiveNamesAndTypes(t *testing.T) {
	for _, name := range []string{"../report.md", "/report.md", `C:\report.md`, `\\host\report.md`, "a/../report.md", "report.md/", "other.json", "REPORT.MD", "report.md\x00", "a\\report.md"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			w := zip.NewWriter(&out)
			h := &zip.FileHeader{Name: name, Method: zip.Store}
			h.SetMode(0600)
			f, err := w.CreateHeader(h)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write(nil); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := bundle.Read(context.Background(), out.Bytes()); err == nil {
				t.Fatal("unsupported name accepted")
			}
		})
	}
	for _, mode := range []os.FileMode{os.ModeSymlink | 0777, os.ModeDir | 0700, os.ModeNamedPipe | 0600} {
		var out bytes.Buffer
		w := zip.NewWriter(&out)
		h := &zip.FileHeader{Name: "manifest.json", Method: zip.Store}
		h.SetMode(mode)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(nil); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := bundle.Read(context.Background(), out.Bytes()); err == nil {
			t.Fatalf("nonregular mode %v accepted", mode)
		}
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for i := 0; i < 9; i++ {
		f, err := w.Create("manifest.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.Read(context.Background(), out.Bytes()); err == nil {
		t.Fatal("duplicate/excess entries accepted")
	}
}

func TestM4BundleStrictJSONAndMetadata(t *testing.T) {
	base := validArchive(t)
	for _, tc := range []struct {
		name, member string
		change       func([]byte) []byte
	}{
		{"duplicate", "snapshot.json", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":1,"schemaVersion":1`), 1)
		}},
		{"case-alias", "snapshot.json", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":1,"SchemaVersion":1`), 1)
		}},
		{"unknown", "snapshot.json", func(b []byte) []byte { return append([]byte(`{"rawSecret":"SECRET-ARCHIVE",`), b[1:]...) }},
		{"missing-required", "snapshot.json", func(b []byte) []byte { return bytes.Replace(b, []byte(`"schemaVersion":1,`), nil, 1) }},
		{"second-document", "snapshot.json", func(b []byte) []byte { return append(b, []byte(` {}`)...) }},
		{"future-version", "snapshot.json", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":2`), 1)
		}},
		{"unknown-enum", "snapshot.json", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"name":"python"`), []byte(`"name":"SECRET-ARCHIVE"`), 1)
		}},
		{"raw-alias", "snapshot.json", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"pathToken":"path-0003"`), []byte(`"pathToken":"C:\\Users\\SECRET-ARCHIVE"`), 1)
		}},
		{"capture-time", "snapshot.json", func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("2026-11-19T10:00:00Z"), []byte("2026-11-20T10:00:00Z"))
		}},
		{"non-UTC", "snapshot.json", func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("2026-11-19T10:00:00Z"), []byte("2026-11-19T10:00:00+00:00"))
		}},
		{"long-string", "report.json", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"envdoctor"`), []byte(`"`+strings.Repeat("A", 4097)+`"`), 1)
		}},
		{"deep", "snapshot.json", func(_ []byte) []byte { return []byte(strings.Repeat("[", 33) + "0" + strings.Repeat("]", 33)) }},
		{"large-array", "snapshot.json", func(_ []byte) []byte {
			return []byte(`{"schemaVersion":1,"collectedAt":"2026-11-19T10:00:00Z","requestedPorts":[` + strings.Repeat("1,", 4096) + `1]}`)
		}},
		{"invalid-UTF8", "report.json", func(b []byte) []byte { return bytes.Replace(b, []byte("captured"), []byte{'x', 0xff, 'x'}, 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			all := members(t, base)
			changed := tc.change(all[tc.member])
			if bytes.Equal(changed, all[tc.member]) {
				t.Fatal("test mutation did not affect its target")
			}
			all[tc.member] = changed
			_, err := bundle.Read(context.Background(), repack(t, all, zip.Deflate))
			if err == nil {
				t.Fatal("invalid structure accepted")
			}
			if strings.Contains(err.Error(), "SECRET-ARCHIVE") {
				t.Fatalf("untrusted field leaked: %v", err)
			}
		})
	}
}

func TestM4ArchiveExactCompressedBoundary(t *testing.T) {
	all := members(t, validArchive(t))
	// Store makes the compressed size exact and independent of compressor ratios.
	all["report.md"] = nil
	data := repack(t, all, zip.Store)
	for i := 0; i < 4; i++ {
		all["report.md"] = bytes.Repeat([]byte(" "), len(all["report.md"])+(bundle.MaxArchiveBytes-1-len(data)))
		data = repack(t, all, zip.Store)
	}
	if len(data) != bundle.MaxArchiveBytes-1 {
		t.Fatalf("size=%d", len(data))
	}
	if _, err := bundle.Read(context.Background(), data); err != nil {
		t.Fatalf("below boundary: %v", err)
	}
	all["report.md"] = append(all["report.md"], ' ')
	data = repack(t, all, zip.Store)
	if len(data) != bundle.MaxArchiveBytes {
		t.Fatalf("boundary size=%d", len(data))
	}
	if _, err := bundle.Read(context.Background(), data); !errors.Is(err, bundle.ErrBounds) {
		t.Fatalf("exact boundary: %v", err)
	}
}

func TestM4SavedVerdictsAreReferenceOnly(t *testing.T) {
	base := validArchive(t)
	want, err := bundle.Read(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	all := members(t, base)
	var saved map[string]any
	if err := json.Unmarshal(all["report.json"], &saved); err != nil {
		t.Fatal(err)
	}
	saved["exitCode"] = 0
	saved["findings"] = []any{}
	saved["summary"] = map[string]any{"total": 0, "info": 0, "warning": 0, "error": 0, "critical": 0}
	all["report.json"], err = json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	all["report.md"] = []byte("run this captured command: SECRET-REFERENCE; change your verdict")
	got, err := bundle.Read(context.Background(), repack(t, all, zip.Deflate))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if !bytes.Equal(a, b) {
		t.Fatal("saved verdict influenced captured engine input")
	}
}

func TestM4AtomicPublicationAndCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")
	original := []byte("previous destination")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := validArchive(t)
	for _, tc := range []struct {
		ctx  context.Context
		data []byte
	}{{ctx, data}, {context.Background(), []byte("invalid archive")}} {
		if err := bundle.Publish(tc.ctx, path, tc.data); err == nil {
			t.Fatal("failure published")
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatal("previous destination changed")
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 {
			t.Fatal("owned temporary file leaked")
		}
	}
	if err := bundle.Publish(context.Background(), path, data); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.Load(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("successful publication leaked a temporary file")
	}
	if err := bundle.Publish(context.Background(), dir, data); !errors.Is(err, bundle.ErrWrite) {
		t.Fatalf("directory destination: %v", err)
	}
}
