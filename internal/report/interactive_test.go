package report

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/shellghostt/envdoctor/internal/collector"
)

func TestM4ViewerNarrowResizeScrollAndInterrupt(t *testing.T) {
	v := newViewer(strings.Repeat("wide 界 text \t evidence\n", 50))
	for _, size := range [][2]int{{1, 1}, {2, 2}, {12, 8}, {80, 24}, {0, 0}, {10000, 10000}} {
		_, _ = v.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, key := range []tea.KeyMsg{{Type: tea.KeyEnd}, {Type: tea.KeyPgUp}, {Type: tea.KeyDown}, {Type: tea.KeyHome}, {Type: tea.KeyUp}, {Type: tea.KeyPgDown}} {
			_, _ = v.Update(key)
		}
		lines := strings.Split(v.View(), "\n")
		if len(lines) > v.height || v.top < 0 || v.top > max(0, len(v.lines)-max(1, v.height-2)) {
			t.Fatalf("viewport out of bounds at %v", size)
		}
		for _, line := range lines {
			if width := ansi.StringWidth(line); width > v.width {
				t.Fatalf("line width=%d viewport=%d: %q", width, v.width, line)
			}
		}
	}
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !v.interrupted || cmd == nil {
		t.Fatal("interrupt did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("interrupt did not return quit command")
	}
}

type lifecycleWriter struct {
	bytes     bytes.Buffer
	cancel    context.CancelFunc
	failFrame bool
	failed    bool
}

func (w *lifecycleWriter) Write(p []byte) (int, error) {
	if w.cancel != nil && bytes.Contains(p, []byte("\x1b[?1049h")) {
		w.cancel()
	}
	if w.failFrame && !w.failed && bytes.Contains(p, []byte("EnvDoctor |")) {
		w.failed = true
		return 0, errors.New("synthetic frame failure")
	}
	return w.bytes.Write(p)
}

func viewerDocument() Document {
	stamp := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	return NewDocument("0.1.0", stamp, collector.Snapshot{SchemaVersion: 1, CollectedAt: stamp, Platform: &collector.Platform{OS: "windows", Arch: "amd64", Version: "11"}}, nil, nil, 12)
}

func TestM4InteractiveQuitCancellationAndOutputCleanup(t *testing.T) {
	for _, name := range []string{"quit", "cancel", "output-failure"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			writer := &lifecycleWriter{}
			var input *strings.Reader
			if name == "quit" {
				input = strings.NewReader("q")
			}
			if name == "cancel" {
				writer.cancel = cancel
			}
			if name == "output-failure" {
				writer.failFrame = true
			}
			var err error
			if input != nil {
				err = WriteInteractive(ctx, input, writer, viewerDocument())
			} else {
				err = WriteInteractive(ctx, nil, writer, viewerDocument())
			}
			if (err != nil) != (name != "quit") {
				t.Fatalf("%s error=%v", name, err)
			}
			if name == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel=%v", err)
			}
			if name == "output-failure" && !writer.failed {
				t.Fatal("frame failure was never exercised")
			}
			output := writer.bytes.String()
			leave := strings.LastIndex(output, "\x1b[?1049l")
			plain := strings.LastIndex(output, "EnvDoctor 0.1.0 - environment diagnosis")
			if !strings.Contains(output, "\x1b[?1049h") || leave < 0 || !strings.Contains(output, "\x1b[?25h") || plain < leave {
				t.Fatalf("screen/cursor/fallback cleanup missing: %q", output)
			}
		})
	}
}

func TestM4RepeatedViewerLifecycleReleasesResources(t *testing.T) {
	doc := viewerDocument()
	if err := WriteInteractive(context.Background(), strings.NewReader("q"), &lifecycleWriter{}, doc); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	beforeG := runtime.NumGoroutine()
	start := time.Now()
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		writer := &lifecycleWriter{}
		if i%2 == 0 {
			if err := WriteInteractive(ctx, strings.NewReader("q"), writer, doc); err != nil {
				t.Fatal(err)
			}
		} else {
			writer.cancel = cancel
			if err := WriteInteractive(ctx, nil, writer, doc); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel=%v", err)
			}
		}
		cancel()
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > beforeG+2 && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	afterG := runtime.NumGoroutine()
	t.Logf("50 viewers: %s; goroutines %d -> %d; retained heap %d -> %d bytes", time.Since(start), beforeG, afterG, before.HeapAlloc, after.HeapAlloc)
	if afterG > beforeG+2 || int64(after.HeapAlloc)-int64(before.HeapAlloc) > 2<<20 {
		t.Fatal("viewer resources retained after repeated exits")
	}
}

func TestM4ResizePollerStopsAndJoinsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	requested := make(chan struct{}, 1)
	done := resizePoller(ctx, func() {
		select {
		case requested <- struct{}{}:
		default:
		}
	}, time.Millisecond)
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("resize was not requested")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("resize timer/goroutine retained")
	}
}
