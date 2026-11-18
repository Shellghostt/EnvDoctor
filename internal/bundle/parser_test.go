package bundle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type deepJSON struct {
	Child *deepJSON `json:"child,omitempty"`
}
type tokenJSON struct {
	Rows [][]int `json:"rows"`
}
type stringJSON struct {
	Value string `json:"value"`
}

func TestM4JSONIndependentWorkBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []byte
		target any
		want   error
	}{
		{"depth", []byte(strings.Repeat(`{"child":`, 32) + `{}` + strings.Repeat(`}`, 32)), &deepJSON{}, ErrBounds},
		{"array", []byte(`{"rows":[[` + strings.Repeat(`1,`, 4096) + `1]]}`), &tokenJSON{}, ErrBounds},
		{"tokens", []byte(`{"rows":[` + strings.Repeat(`[`+strings.Repeat(`1,`, 4095)+`1],`, 50) + `[` + strings.Repeat(`1,`, 4095) + `1]]}`), &tokenJSON{}, ErrBounds},
		{"UTF8-byte-length", []byte(`{"value":"` + strings.Repeat("界", 1366) + `"}`), &stringJSON{}, ErrBounds},
		{"escaped-key-duplicate", []byte(`{"value":"one","\u0076alue":"two"}`), &stringJSON{}, ErrInvalid},
		{"case-alias", []byte(`{"Value":"one"}`), &stringJSON{}, ErrInvalid},
		{"null-required", []byte(`{"value":null}`), &stringJSON{}, ErrInvalid},
		{"missing-required", []byte(`{}`), &stringJSON{}, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := decodeJSON(context.Background(), tc.data, tc.target); !errors.Is(err, tc.want) {
				t.Fatalf("got=%v want=%v", err, tc.want)
			}
		})
	}
	data, _ := json.Marshal(stringJSON{Value: strings.Repeat("A", 4096)})
	if err := decodeJSON(context.Background(), data, &stringJSON{}); err != nil {
		t.Fatalf("exact string boundary: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := decodeJSON(ctx, data, &stringJSON{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}

func TestM4ArchiveBufferCannotBypassSizeGuard(t *testing.T) {
	out := &archiveBuffer{ctx: context.Background()}
	if _, ok := any(out).(io.ReaderFrom); ok {
		t.Fatal("unbounded ReaderFrom exposed")
	}
	_, err := io.Copy(out, bytes.NewReader(bytes.Repeat([]byte{0}, MaxArchiveBytes)))
	if !errors.Is(err, ErrBounds) || out.Len() >= MaxArchiveBytes {
		t.Fatalf("guard bypassed: size=%d error=%v", out.Len(), err)
	}
}
