package bundle_test

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"hash/crc32"
	"runtime"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/bundle"
)

func rawArchive(t *testing.T, all map[string][]byte, method uint16, corrupt string) []byte {
	t.Helper()
	// Repack first to produce a matching manifest. CreateRaw then exercises the
	// valid no-data-descriptor ZIP form independently of Go's default writer.
	all = members(t, repack(t, all, zip.Store))
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, name := range []string{"manifest.json", "snapshot.json", "report.json", "report.md", "versions.json", "diagnostics.json"} {
		body, ok := all[name]
		if !ok {
			continue
		}
		compressed := body
		if method == zip.Deflate {
			var b bytes.Buffer
			f, err := flate.NewWriter(&b, flate.DefaultCompression)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write(body); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			compressed = b.Bytes()
		}
		if name == corrupt {
			compressed = append(bytes.Clone(compressed), 0)
		}
		h := &zip.FileHeader{Name: name, Method: method, ReaderVersion: 20, CreatorVersion: 3<<8 | 20, CRC32: crc32.ChecksumIEEE(body), CompressedSize64: uint64(len(compressed)), UncompressedSize64: uint64(len(body))}
		h.SetMode(0600)
		stream, err := w.CreateRaw(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stream.Write(compressed); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestM4StoreDeflateAndCompressedTrailingGarbage(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		data := rawArchive(t, members(t, validArchive(t)), method, "")
		if _, err := bundle.Read(context.Background(), data); err != nil {
			t.Fatalf("method %d: %v", method, err)
		}
	}
	data := rawArchive(t, members(t, validArchive(t)), zip.Deflate, "snapshot.json")
	if _, err := bundle.Read(context.Background(), data); err == nil {
		t.Fatal("trailing compressed bytes accepted")
	}
}

func TestM4DeclaredAndHiddenDecompressionBombAreBounded(t *testing.T) {
	all := members(t, validArchive(t))
	all["snapshot.json"] = bytes.Repeat([]byte(" "), 8<<20)
	data := rawArchive(t, all, zip.Deflate, "")
	if _, err := bundle.Read(context.Background(), data); err == nil {
		t.Fatal("oversized declared snapshot accepted")
	}
	// Lie consistently in local+central sizes. The actual streamed output must
	// still stop at the declared bound rather than allocate the 8 MiB expansion.
	pos := central(data)
	for i := 0; i < 5; i++ {
		nameLen := int(binary.LittleEndian.Uint16(data[pos+28:]))
		if string(data[pos+46:pos+46+nameLen]) == "snapshot.json" {
			local := int(binary.LittleEndian.Uint32(data[pos+42:]))
			binary.LittleEndian.PutUint32(data[pos+24:], 0)
			binary.LittleEndian.PutUint32(data[local+22:], 0)
			break
		}
		pos += 46 + nameLen
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	for i := 0; i < 25; i++ {
		if _, err := bundle.Read(context.Background(), data); err == nil {
			t.Fatal("hidden decompression bomb accepted")
		}
	}
	runtime.ReadMemStats(&after)
	t.Logf("25 hidden bombs: %s; total allocated %d bytes", time.Since(start), after.TotalAlloc-before.TotalAlloc)
	if after.TotalAlloc-before.TotalAlloc > 8<<20 {
		t.Fatal("rejection allocated the hidden expansions")
	}
}

func TestM4DuplicateEntryBelowEntryLimit(t *testing.T) {
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, name := range []string{"manifest.json", "snapshot.json", "report.json", "report.md", "versions.json", "report.md"} {
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(0600)
		stream, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stream.Write([]byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.Read(context.Background(), out.Bytes()); err == nil {
		t.Fatal("duplicate member accepted below max entry count")
	}
}
