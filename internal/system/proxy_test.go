package system

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestProxyProbeSendsNoApplicationBytesAndClosesConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var data [1]byte
		n, err := conn.Read(data[:])
		if n != 0 || !errors.Is(err, io.EOF) {
			result <- errors.New("proxy probe sent bytes or failed to close its connection")
			return
		}
		result <- nil
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	status, err := ProbeProxy(ctx, "127.0.0.1", listener.Addr().(*net.TCPAddr).Port)
	if err != nil || status != "reachable" {
		t.Fatalf("probe=%s err=%v", status, err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connection did not terminate")
	}
}

func TestProxyRefusalIsConclusiveButCancellationIsUnknown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	status, err := ProbeProxy(ctx, "127.0.0.1", port)
	if err != nil || status != "unreachable" {
		t.Fatalf("refusal became ambiguous: %s %v", status, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	status, err = ProbeProxy(ctx, "127.0.0.1", port)
	if status != "unknown" || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation became outage evidence: %s %v", status, err)
	}
}

func TestProxyDNSIsBoundedByCancellation(t *testing.T) {
	old := net.DefaultResolver
	defer func() { net.DefaultResolver = old }()
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) { <-ctx.Done(); return nil, ctx.Err() }}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	status, err := ProbeProxy(ctx, "synthetic-proxy.invalid", 8888)
	if status != "unknown" || err == nil || time.Since(start) > time.Second {
		t.Fatalf("DNS escaped deadline: %s %v %s", status, err, time.Since(start))
	}
}

func TestRepeatedProxyLifecycleReleasesResources(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	defer func() { _ = listener.Close(); <-finished }()
	runtime.GC()
	beforeG := runtime.NumGoroutine()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		status, err := ProbeProxy(ctx, "127.0.0.1", listener.Addr().(*net.TCPAddr).Port)
		cancel()
		if err != nil || status != "reachable" {
			t.Fatalf("cycle %d: %s %v", i, status, err)
		}
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	<-finished
	runtime.GC()
	runtime.ReadMemStats(&after)
	afterG := runtime.NumGoroutine()
	if afterG > beforeG+4 {
		t.Fatalf("goroutines retained: before=%d after=%d", beforeG, afterG)
	}
	t.Logf("100 cycles in %s; goroutines %d -> %d; retained heap %d -> %d bytes", time.Since(start), beforeG, afterG, before.HeapAlloc, after.HeapAlloc)
}

func BenchmarkProxyLifecycle(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	b.Cleanup(func() { _ = listener.Close(); <-finished })
	port := listener.Addr().(*net.TCPAddr).Port
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		status, err := ProbeProxy(ctx, "127.0.0.1", port)
		cancel()
		if err != nil || status != "reachable" {
			b.Fatalf("probe: %s %v", status, err)
		}
	}
}

func TestNativeExecutableDoesNotTreatTextShimsAsResolvedInterpreters(t *testing.T) {
	file := filepath.Join(t.TempDir(), "python")
	if err := os.WriteFile(file, []byte("#!/bin/sh\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if known, err := NativeExecutable(file, runtime.GOOS); err != nil || known {
		t.Fatalf("text shim classified as native: %v %v", known, err)
	}
	if known, err := NativeExecutable(os.Args[0], runtime.GOOS); err != nil || !known {
		t.Fatalf("native file header not recognized: %v %v", known, err)
	}
}
