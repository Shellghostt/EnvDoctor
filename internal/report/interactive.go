package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type viewer struct {
	text        string
	lines       []string
	width       int
	height      int
	top         int
	interrupted bool
}

func newViewer(text string) *viewer {
	v := &viewer{text: strings.ReplaceAll(text, "\t", "    "), width: 80, height: 24}
	v.resize(80, 24)
	return v
}

func (v *viewer) resize(width, height int) {
	v.width, v.height = max(1, min(width, 512)), max(1, min(height, 256))
	v.lines = strings.Split(ansi.Hardwrap(v.text, v.width, false), "\n")
	v.top = min(v.top, max(0, len(v.lines)-max(1, v.height-2)))
}

func (*viewer) Init() tea.Cmd { return nil }

func (v *viewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if max(1, min(msg.Width, 512)) != v.width || max(1, min(msg.Height, 256)) != v.height {
			v.resize(msg.Width, msg.Height)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return v, tea.Quit
		case "ctrl+c":
			v.interrupted = true
			return v, tea.Quit
		case "up", "k":
			v.top--
		case "down", "j":
			v.top++
		case "pgup":
			v.top -= max(1, v.height-2)
		case "pgdown", " ":
			v.top += max(1, v.height-2)
		case "home", "g":
			v.top = 0
		case "end", "G":
			v.top = len(v.lines)
		}
		v.top = max(0, min(v.top, max(0, len(v.lines)-max(1, v.height-2))))
	}
	return v, nil
}

func (v *viewer) View() string {
	header := ansi.Truncate("EnvDoctor | captured facts and manual advice", v.width, "")
	if v.height == 1 {
		return header
	}
	footer := ansi.Truncate(fmt.Sprintf("↑/↓ scroll · pgup/down · q exit | %d/%d", v.top+1, len(v.lines)), v.width, "")
	if v.height == 2 {
		return header + "\n" + footer
	}
	end := min(len(v.lines), v.top+v.height-2)
	lines := make([]string, 0, end-v.top)
	for _, line := range v.lines[v.top:end] {
		line = ansi.Truncate(line, v.width, "")
		for _, style := range []struct{ prefix, color string }{{"[CRITICAL]", "35"}, {"[ERROR]", "31"}, {"[WARNING]", "33"}, {"[INFO]", "34"}} {
			if strings.HasPrefix(line, style.prefix) {
				line = "\x1b[1;" + style.color + "m" + line + "\x1b[0m"
				break
			}
		}
		lines = append(lines, line)
	}
	return "\x1b[1;36m" + header + "\x1b[0m\n" + strings.Join(lines, "\n") + "\n\x1b[2m" + footer + "\x1b[0m"
}

// Bubble Tea's renderer can ignore Write errors. Record them and cancel its
// context so an output failure cannot leave an interactive event loop running.
type terminalWriter struct {
	output io.Writer
	cancel context.CancelFunc
	mu     sync.Mutex
	err    error
}

func (w *terminalWriter) Fd() uintptr {
	if file, ok := w.output.(interface{ Fd() uintptr }); ok {
		return file.Fd()
	}
	return ^uintptr(0)
}

func (w *terminalWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.output.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		if w.err == nil {
			w.err = err
		}
		w.cancel()
	}
	return n, err
}

type terminalState struct {
	fd    int
	state *term.State
}

func saveTerminalStates(streams ...any) ([]terminalState, error) {
	states := []terminalState{}
	seen := map[int]bool{}
	for _, stream := range streams {
		file, ok := stream.(interface{ Fd() uintptr })
		if !ok {
			continue
		}
		fd := int(file.Fd())
		if seen[fd] || !term.IsTerminal(fd) {
			continue
		}
		state, err := term.GetState(fd)
		if err != nil {
			return nil, err
		}
		states = append(states, terminalState{fd: fd, state: state})
		seen[fd] = true
	}
	return states, nil
}

func restoreTerminalStates(states []terminalState) error {
	var result error
	for _, saved := range states {
		result = errors.Join(result, term.Restore(saved.fd, saved.state))
	}
	return result
}

func resizePoller(ctx context.Context, request func(), interval time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				request()
			}
		}
	}()
	return done
}

func (w *terminalWriter) failure() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// WriteInteractive always leaves a plain report after the terminal viewer.
// The caller selects this path only for capable terminal streams.
func WriteInteractive(ctx context.Context, input io.Reader, output io.Writer, doc Document) error {
	var plain bytes.Buffer
	if err := WriteText(&plain, doc); err != nil {
		return err
	}
	states, saveErr := saveTerminalStates(input, output)
	if saveErr != nil {
		return errors.Join(saveErr, WriteText(output, doc))
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &terminalWriter{output: output, cancel: cancel}
	v := newViewer(plain.String())
	program := tea.NewProgram(v, tea.WithInput(input), tea.WithOutput(w), tea.WithContext(runCtx),
		tea.WithAltScreen(), tea.WithoutSignalHandler(), tea.WithoutBracketedPaste())
	// Bubble Tea v1.3.10 only checks the initial size on Windows (no SIGWINCH).
	// Poll through its window-size command on terminal outputs. The adapter owns
	// and joins this context-cancelled timer on every return, including startup
	// and output failure; no Tick command remains asleep after the viewer exits.
	var resizeDone <-chan struct{}
	if file, ok := output.(interface{ Fd() uintptr }); ok && term.IsTerminal(int(file.Fd())) {
		resizeDone = resizePoller(runCtx, func() { program.Send(tea.WindowSize()()) }, 200*time.Millisecond)
	}
	_, runErr := program.Run()
	cancel()
	if resizeDone != nil {
		<-resizeDone
	}
	var cleanupErr error
	if runErr != nil && input != nil {
		cleanupErr = program.ReleaseTerminal()
	}
	cleanupErr = errors.Join(cleanupErr, restoreTerminalStates(states))
	if v.interrupted {
		runErr = errors.Join(runErr, context.Canceled)
	}
	writeErr := WriteText(output, doc)
	if err := errors.Join(runErr, cleanupErr, w.failure(), writeErr); err != nil {
		return fmt.Errorf("interactive report failed; plain fallback attempted: %w", err)
	}
	return nil
}
