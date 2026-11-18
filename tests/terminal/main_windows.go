//go:build windows

// Command terminal is a manual native-console evidence harness. It calls the
// production CLI with real handles and can inject one transient frame failure.
// It never changes collectors/rules or adds a production diagnostic flag.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/shellghostt/envdoctor/internal/cli"
	"golang.org/x/sys/windows"
)

type consoleState struct {
	InputMode     uint32 `json:"inputMode"`
	OutputMode    uint32 `json:"outputMode"`
	CursorVisible bool   `json:"cursorVisible"`
}
type evidence struct {
	ExitCode             int             `json:"exitCode"`
	Before               consoleState    `json:"before"`
	After                consoleState    `json:"after"`
	StateRestored        bool            `json:"stateRestored"`
	FrameFailureInjected bool            `json:"frameFailureInjected"`
	Status               string          `json:"status"`
	ResizeAttempts       []resizeAttempt `json:"resizeAttempts,omitempty"`
}

type resizeAttempt struct {
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	Applied bool `json:"applied"`
}

func resizeConsole(width, height int) bool {
	buffer := windows.Handle(os.Stdout.Fd())
	var info windows.ConsoleScreenBufferInfo
	if windows.GetConsoleScreenBufferInfo(buffer, &info) != nil {
		return false
	}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	windowProc, bufferProc := kernel.NewProc("SetConsoleWindowInfo"), kernel.NewProc("SetConsoleScreenBufferSize")
	// A smaller viewport does not require shrinking ConPTY's backing buffer.
	// Grow that buffer only when needed, then set the visible rectangle. This
	// avoids rejected buffer shrinks and affects only this owned console.
	if width > int(info.Size.X) || height > int(info.Size.Y) {
		bufferWidth, bufferHeight := max(width, int(info.Size.X)), max(height, int(info.Size.Y))
		ok, _, _ := bufferProc.Call(uintptr(buffer), uintptr(uint32(uint16(bufferWidth))|uint32(uint16(bufferHeight))<<16))
		if ok == 0 {
			return false
		}
	}
	window := windows.SmallRect{Right: int16(width - 1), Bottom: int16(height - 1)}
	ok, _, _ := windowProc.Call(uintptr(buffer), 1, uintptr(unsafe.Pointer(&window)))
	return ok != 0
}

func consoleResizeSequence(ctx context.Context) <-chan []resizeAttempt {
	done := make(chan []resizeAttempt, 1)
	go func() {
		attempts := []resizeAttempt{}
		defer func() { done <- attempts }()
		for _, size := range [][2]int{{40, 18}, {96, 28}} {
			timer := time.NewTimer(12 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			attempts = append(attempts, resizeAttempt{Width: size[0], Height: size[1], Applied: resizeConsole(size[0], size[1])})
		}
	}()
	return done
}

type frameWriter struct {
	file           *os.File
	inject, failed bool
}

func (w *frameWriter) Fd() uintptr { return w.file.Fd() }
func (w *frameWriter) Write(p []byte) (int, error) {
	if w.inject && !w.failed && bytes.Contains(p, []byte("EnvDoctor |")) {
		w.failed = true
		return 0, errors.New("synthetic native frame write failure")
	}
	return w.file.Write(p)
}

func state() (consoleState, error) {
	var s consoleState
	if err := windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &s.InputMode); err != nil {
		return s, err
	}
	if err := windows.GetConsoleMode(windows.Handle(os.Stdout.Fd()), &s.OutputMode); err != nil {
		return s, err
	}
	var cursor struct {
		Size    uint32
		Visible int32
	}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleCursorInfo")
	ok, _, err := proc.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&cursor)))
	if ok == 0 {
		return s, err
	}
	s.CursorVisible = cursor.Visible != 0
	return s, nil
}

func main() {
	flags := flag.NewFlagSet("terminal-evidence", flag.ExitOnError)
	path := flags.String("record", "bin/m4-native-terminal.json", "owned evidence destination")
	inject := flags.Bool("fail-frame", false, "inject one transient frame write error")
	resize := flags.Bool("resize", false, "exercise two owned native console sizes")
	_ = flags.Parse(os.Args[1:])
	before, beforeErr := state()
	w := &frameWriter{file: os.Stdout, inject: *inject}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	var resizeDone <-chan []resizeAttempt
	if *resize {
		resizeDone = consoleResizeSequence(ctx)
	}
	code := cli.Main(append([]string{"envdoctor"}, flags.Args()...), cli.App{Version: "m4-local", Stdin: os.Stdin, Stdout: w, Stderr: os.Stderr, Context: ctx})
	stop()
	after, afterErr := state()
	record := evidence{ExitCode: code, Before: before, After: after, StateRestored: beforeErr == nil && afterErr == nil && before == after, FrameFailureInjected: w.failed, Status: "native console"}
	if resizeDone != nil {
		record.ResizeAttempts = <-resizeDone
	}
	if beforeErr != nil || afterErr != nil {
		record.Status = "native console state unavailable"
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil || os.WriteFile(*path, append(data, '\n'), 0600) != nil {
		os.Exit(3)
	}
	os.Exit(code)
}
