package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/config"
)

const maxSnapshotBytes = 8 << 20

func loadSnapshot(dir string) (collector.Snapshot, error) {
	file, err := os.Open(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		return collector.Snapshot{}, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxSnapshotBytes+1))
	if err != nil {
		return collector.Snapshot{}, err
	}
	if len(data) > maxSnapshotBytes {
		return collector.Snapshot{}, fmt.Errorf("snapshot exceeds 8 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var snapshot collector.Snapshot
	if err = decoder.Decode(&snapshot); err != nil {
		return collector.Snapshot{}, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return collector.Snapshot{}, fmt.Errorf("snapshot must contain exactly one JSON document")
	}
	if snapshot.SchemaVersion != config.SchemaVersion {
		return collector.Snapshot{}, fmt.Errorf("unsupported snapshot version")
	}
	if snapshot.CollectedAt.IsZero() {
		return collector.Snapshot{}, fmt.Errorf("snapshot has no collectedAt timestamp")
	}
	return snapshot, nil
}
