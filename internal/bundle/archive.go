package bundle

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"io"
	"sort"
	"unicode/utf8"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

type archiveBuffer struct {
	ctx    context.Context
	buffer bytes.Buffer
}

func (b *archiveBuffer) Len() int      { return b.buffer.Len() }
func (b *archiveBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *archiveBuffer) Write(data []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) >= MaxArchiveBytes-b.Len() {
		return 0, ErrBounds
	}
	return b.buffer.Write(data)
}

func marshalMember(ctx context.Context, name string, value any) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, ErrEvidence
	}
	if len(data) > memberLimits[name] {
		return nil, ErrBounds
	}
	return data, nil
}

// Build recomputes all report members from the sharing model with the builtin
// engine. It never zips the original document, findings, errors or log text.
func Build(ctx context.Context, original report.Document, opts Options) ([]byte, error) {
	snapshot, source, timings, diags, err := export(ctx, original)
	if err != nil {
		return nil, err
	}
	if err := validateSnapshot(snapshot); err != nil {
		return nil, err
	}
	evidence := engineSnapshot(snapshot)
	findings, err := diagnostic.DefaultEngine().Evaluate(evidence, diagnostic.Filter{})
	if err != nil {
		return nil, ErrEvidence
	}
	doc := report.NewDocument(source.CaptureTool.Version, source.CapturedAt, evidence, findings, timings, len(diagnostic.BuiltinMetadata()))
	doc.Source = &source
	all := map[string][]byte{}
	for _, member := range []struct {
		name  string
		value any
	}{{"snapshot.json", snapshot}, {"report.json", doc}, {"versions.json", makeVersions(snapshot, source.CaptureTool)}} {
		all[member.name], err = marshalMember(ctx, member.name, member.value)
		if err != nil {
			return nil, err
		}
	}
	var markdown bytes.Buffer
	if err := report.WriteMarkdown(&markdown, doc); err != nil {
		return nil, ErrBounds
	}
	all["report.md"] = markdown.Bytes()
	if opts.IncludeLogs {
		all["diagnostics.json"], err = marshalMember(ctx, "diagnostics.json", diags)
		if err != nil {
			return nil, err
		}
	}
	manifest := Manifest{BundleVersion: 1, SnapshotVersion: 1, ReportVersion: 1, RedactionPolicy: Policy, Tool: source.CaptureTool, CapturedAt: source.CapturedAt, CaptureExitCode: source.CaptureExitCode, Omissions: source.Omissions, Members: []Member{}}
	names := []string{"manifest.json", "snapshot.json", "report.json", "report.md", "versions.json"}
	if opts.IncludeLogs {
		names = append(names, "diagnostics.json")
	}
	total := 0
	for _, name := range names[1:] {
		body := all[name]
		digest := sha256.Sum256(body)
		manifest.Members = append(manifest.Members, Member{Name: name, Size: len(body), SHA256: hex.EncodeToString(digest[:])})
		total += len(body)
	}
	all["manifest.json"], err = marshalMember(ctx, "manifest.json", manifest)
	if err != nil {
		return nil, err
	}
	if total+len(all["manifest.json"]) > MaxUncompressedBytes {
		return nil, ErrBounds
	}
	out := archiveBuffer{ctx: ctx}
	w := zip.NewWriter(&out)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		// No extended timestamp/extra/comment records or platform paths.
		header.SetMode(0600)
		stream, err := w.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := stream.Write(all[name]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	data := out.Bytes()
	if _, err := Read(ctx, data); err != nil {
		return nil, err
	}
	return data, nil
}

type zipEntry struct {
	name                                string
	offset, compressed, uncompressed    int
	crc                                 uint32
	flags, method                       uint16
	version, modifiedTime, modifiedDate uint16
}

// Preflight the entire directory and every local record BEFORE archive/zip can
// allocate its file graph. Offsets must describe one exact nonoverlapping ZIP,
// without a prefix, trailing records, extras, ZIP64 or another disk.
func preflight(data []byte) ([]zipEntry, error) {
	if len(data) >= MaxArchiveBytes {
		return nil, ErrBounds
	}
	if len(data) < 22 {
		return nil, ErrInvalid
	}
	eocd := len(data) - 22
	end := data[eocd:]
	u16 := binary.LittleEndian.Uint16
	u32 := binary.LittleEndian.Uint32
	if u32(end) != 0x06054b50 || u16(end[4:]) != 0 || u16(end[6:]) != 0 || u16(end[20:]) != 0 {
		return nil, ErrInvalid
	}
	count := int(u16(end[10:]))
	if count > MaxEntries {
		return nil, ErrBounds
	}
	if count < 5 || count != int(u16(end[8:])) {
		return nil, ErrInvalid
	}
	dirSize, dirStart := uint64(u32(end[12:])), uint64(u32(end[16:]))
	if dirStart+dirSize != uint64(eocd) || dirStart > uint64(eocd) {
		return nil, ErrInvalid
	}
	position := int(dirStart)
	entries := make([]zipEntry, 0, count)
	seen := map[string]bool{}
	total := 0
	for i := 0; i < count; i++ {
		if position > eocd-46 {
			return nil, ErrInvalid
		}
		h := data[position : position+46]
		if u32(h) != 0x02014b50 || u16(h[6:]) != 10 && u16(h[6:]) != 20 || u16(h[34:]) != 0 {
			return nil, ErrInvalid
		}
		flags, method := u16(h[8:]), u16(h[10:])
		if flags & ^uint16(0x0808) != 0 || method != zip.Store && method != zip.Deflate {
			return nil, ErrInvalid
		}
		if method == zip.Deflate && u16(h[6:]) != 20 {
			return nil, ErrInvalid
		}
		nameSize, extraSize, commentSize := int(u16(h[28:])), int(u16(h[30:])), int(u16(h[32:]))
		if extraSize != 0 || commentSize != 0 || nameSize < 1 || nameSize > 16 || position+46+nameSize > eocd {
			return nil, ErrInvalid
		}
		name := string(data[position+46 : position+46+nameSize])
		limit, ok := memberLimits[name]
		if !ok || seen[name] {
			return nil, ErrInvalid
		}
		seen[name] = true
		creator := u16(h[4:])
		if creator>>8 != 0 && creator>>8 != 3 {
			return nil, ErrInvalid
		}
		mode := (&zip.FileHeader{Name: name, CreatorVersion: creator, ExternalAttrs: u32(h[38:])}).Mode()
		if !mode.IsRegular() {
			return nil, ErrInvalid
		}
		attrs := u32(h[38:])
		if creator>>8 == 0 && attrs>>16 != 0 || creator>>8 == 3 && attrs>>28 != 0 && attrs>>28 != 8 || attrs&0x18 != 0 {
			return nil, ErrInvalid
		}
		compressed, uncompressed, offset := uint64(u32(h[20:])), uint64(u32(h[24:])), uint64(u32(h[42:]))
		if uncompressed > uint64(limit) || compressed >= MaxArchiveBytes {
			return nil, ErrBounds
		}
		if method == zip.Store && compressed != uncompressed {
			return nil, ErrIntegrity
		}
		if offset > dirStart || offset+30+uint64(nameSize)+compressed > dirStart {
			return nil, ErrInvalid
		}
		total += int(uncompressed)
		if total > MaxUncompressedBytes {
			return nil, ErrBounds
		}
		entries = append(entries, zipEntry{name: name, offset: int(offset), compressed: int(compressed), uncompressed: int(uncompressed), crc: u32(h[16:]), flags: flags, method: method, version: u16(h[6:]), modifiedTime: u16(h[12:]), modifiedDate: u16(h[14:])})
		position += 46 + nameSize
	}
	if position != eocd {
		return nil, ErrInvalid
	}
	for _, name := range requiredMembers {
		if !seen[name] {
			return nil, ErrInvalid
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].offset < entries[j].offset })
	position = 0
	for _, entry := range entries {
		if entry.offset != position || position > int(dirStart)-30 {
			return nil, ErrInvalid
		}
		h := data[position : position+30]
		if u32(h) != 0x04034b50 || u16(h[4:]) != entry.version || u16(h[6:]) != entry.flags || u16(h[8:]) != entry.method || u16(h[10:]) != entry.modifiedTime || u16(h[12:]) != entry.modifiedDate || int(u16(h[26:])) != len(entry.name) || u16(h[28:]) != 0 {
			return nil, ErrInvalid
		}
		if string(data[position+30:position+30+len(entry.name)]) != entry.name {
			return nil, ErrInvalid
		}
		if entry.flags&8 == 0 {
			if u32(h[14:]) != entry.crc || u32(h[18:]) != uint32(entry.compressed) || u32(h[22:]) != uint32(entry.uncompressed) {
				return nil, ErrIntegrity
			}
		} else {
			if u32(h[14:]) != 0 && u32(h[14:]) != entry.crc || u32(h[18:]) != 0 && u32(h[18:]) != uint32(entry.compressed) || u32(h[22:]) != 0 && u32(h[22:]) != uint32(entry.uncompressed) {
				return nil, ErrIntegrity
			}
		}
		position += 30 + len(entry.name) + entry.compressed
		if entry.flags&8 != 0 {
			if position > int(dirStart)-12 {
				return nil, ErrInvalid
			}
			if u32(data[position:]) == 0x08074b50 {
				position += 4
			}
			if position > int(dirStart)-12 {
				return nil, ErrInvalid
			}
			if u32(data[position:]) != entry.crc || u32(data[position+4:]) != uint32(entry.compressed) || u32(data[position+8:]) != uint32(entry.uncompressed) {
				return nil, ErrIntegrity
			}
			position += 12
		}
	}
	if position != int(dirStart) {
		return nil, ErrInvalid
	}
	return entries, nil
}

type contextReader struct {
	ctx   context.Context
	input io.Reader
}

// Implement flate.Reader's byte method so the inflater cannot read ahead and
// hide garbage after a complete Deflate stream inside its compressed extent.
type compressedReader struct {
	ctx    context.Context
	reader *bytes.Reader
}

func (r compressedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
func (r compressedReader) ReadByte() (byte, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.ReadByte()
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.input.Read(data)
}

// Read validates all archive/reference members but returns only typed evidence
// and provenance. No saved verdict, Markdown, command or URL is returned.
func Read(ctx context.Context, data []byte) (Capture, error) {
	if err := ctx.Err(); err != nil {
		return Capture{}, err
	}
	entries, err := preflight(data)
	if err != nil {
		return Capture{}, err
	}
	all := make(map[string][]byte, len(entries))
	total := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return Capture{}, err
		}
		start := entry.offset + 30 + len(entry.name)
		compressed := compressedReader{ctx: ctx, reader: bytes.NewReader(data[start : start+entry.compressed])}
		stream := io.NopCloser(io.Reader(compressed))
		if entry.method == zip.Deflate {
			stream = flate.NewReader(compressed)
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, int64(entry.uncompressed)+1))
		closeErr := stream.Close()
		if err := ctx.Err(); err != nil {
			return Capture{}, err
		}
		if len(body) > memberLimits[entry.name] || total+len(body) > MaxUncompressedBytes {
			return Capture{}, ErrBounds
		}
		if readErr != nil || closeErr != nil || len(body) != entry.uncompressed || compressed.reader.Len() != 0 || crc32.ChecksumIEEE(body) != entry.crc {
			return Capture{}, ErrIntegrity
		}
		all[entry.name] = body
		total += len(body)
	}
	var manifest Manifest
	if !utf8.Valid(all["report.md"]) {
		return Capture{}, ErrInvalid
	}
	if err := decodeJSON(ctx, all["manifest.json"], &manifest); err != nil {
		return Capture{}, err
	}
	if err := validateManifest(manifest, all); err != nil {
		return Capture{}, err
	}
	var snapshot SharingSnapshot
	if err := decodeJSON(ctx, all["snapshot.json"], &snapshot); err != nil {
		return Capture{}, err
	}
	if err := validateSnapshot(snapshot); err != nil {
		return Capture{}, err
	}
	var doc report.Document
	if err := decodeJSON(ctx, all["report.json"], &doc); err != nil {
		return Capture{}, err
	}
	var versions Versions
	if err := decodeJSON(ctx, all["versions.json"], &versions); err != nil {
		return Capture{}, err
	}
	var diags *Diagnostics
	if data, ok := all["diagnostics.json"]; ok {
		diags = &Diagnostics{}
		if err := decodeJSON(ctx, data, diags); err != nil {
			return Capture{}, err
		}
	}
	if err := validateReference(doc, snapshot, manifest, versions, diags); err != nil {
		return Capture{}, err
	}
	return Capture{Snapshot: engineSnapshot(snapshot), Source: sourceFor(manifest)}, nil
}
