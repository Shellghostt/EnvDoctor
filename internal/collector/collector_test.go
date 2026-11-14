package collector_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
)

type fakeCollector struct {
	name  string
	part  collector.Part
	err   error
	delay time.Duration
}

type stuckCollector struct{ release <-chan struct{} }

func (stuckCollector) Name() string { return "stuck" }
func (c stuckCollector) Collect(context.Context) (collector.Part, error) {
	<-c.release
	return collector.Part{}, nil
}

func TestRunnerEnforcesDeadlineWhenCollectorIgnoresCancellation(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	done := make(chan []collector.Timing, 1)
	go func() {
		_, timings := (collector.Runner{Collectors: []collector.Collector{stuckCollector{release}}, Timeout: 20 * time.Millisecond}).Run(context.Background())
		done <- timings
	}()
	select {
	case timings := <-done:
		if !collector.Degraded(timings) {
			t.Fatal("timeout was not recorded")
		}
	case <-time.After(time.Second):
		t.Fatal("collector bypassed the runner deadline")
	}
}

func (f fakeCollector) Name() string { return f.name }

func (f fakeCollector) Collect(ctx context.Context) (collector.Part, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return collector.Part{}, ctx.Err()
		}
	}
	return f.part, f.err
}

func TestRunnerMergesParts(t *testing.T) {
	snapshot, timings := collector.Runner{
		Collectors: []collector.Collector{
			fakeCollector{name: "platform", part: collector.Part{Platform: &collector.Platform{
				OS: "linux", Version: "test", Arch: "amd64",
			}}},
			fakeCollector{name: "shell", part: collector.Part{Shell: &collector.Shell{Name: "bash"}}},
		},
		Now: func() time.Time { return time.Unix(1763000000, 0) },
	}.Run(context.Background())

	if snapshot.SchemaVersion != 1 {
		t.Errorf("schema version = %d", snapshot.SchemaVersion)
	}
	if !snapshot.CollectedAt.Equal(time.Unix(1763000000, 0).UTC()) {
		t.Errorf("collectedAt = %s", snapshot.CollectedAt)
	}
	if snapshot.Platform == nil || snapshot.Platform.OS != "linux" {
		t.Fatalf("platform not merged: %+v", snapshot.Platform)
	}
	if snapshot.Shell == nil || snapshot.Shell.Name != "bash" {
		t.Fatalf("shell not merged: %+v", snapshot.Shell)
	}
	if len(timings) != 2 || timings[0].Name != "platform" || timings[1].Name != "shell" {
		t.Fatalf("timings are not in registration order: %+v", timings)
	}
	if collector.Degraded(timings) {
		t.Fatal("healthy collectors reported as degraded")
	}
}

func TestRunnerRunsCollectorsInParallel(t *testing.T) {
	delay := 200 * time.Millisecond

	start := time.Now()
	_, timings := collector.Runner{
		Collectors: []collector.Collector{
			fakeCollector{name: "a", delay: delay},
			fakeCollector{name: "b", delay: delay},
			fakeCollector{name: "c", delay: delay},
		},
		Timeout: 2 * time.Second,
	}.Run(context.Background())
	elapsed := time.Since(start)

	if elapsed > 2*delay {
		t.Fatalf("three %s collectors took %s, which is not parallel", delay, elapsed)
	}
	if collector.Degraded(timings) {
		t.Fatal("unexpected degraded result")
	}
}

func TestRunnerRecordsTimeoutAndContinues(t *testing.T) {
	snapshot, timings := collector.Runner{
		Collectors: []collector.Collector{
			fakeCollector{name: "slow", delay: time.Second},
			fakeCollector{name: "fast", part: collector.Part{Shell: &collector.Shell{Name: "zsh"}}},
		},
		Timeout: 50 * time.Millisecond,
	}.Run(context.Background())

	if !collector.Degraded(timings) {
		t.Fatal("timeout was not reported")
	}
	if len(snapshot.Errors) != 1 {
		t.Fatalf("errors = %+v", snapshot.Errors)
	}
	if snapshot.Errors[0].Collector != "slow" {
		t.Errorf("degraded collector = %q", snapshot.Errors[0].Collector)
	}
	if snapshot.Errors[0].Message != "timeout after 50ms" {
		t.Errorf("message = %q", snapshot.Errors[0].Message)
	}
	if snapshot.Shell == nil {
		t.Fatal("a slow collector must not drop the rest of the snapshot")
	}
}

func TestRunnerRecordsCollectorError(t *testing.T) {
	snapshot, timings := collector.Runner{
		Collectors: []collector.Collector{
			fakeCollector{name: "path", err: errors.New("permission denied")},
		},
	}.Run(context.Background())

	if len(snapshot.Errors) != 1 || snapshot.Errors[0].Message != "permission denied" {
		t.Fatalf("errors = %+v", snapshot.Errors)
	}
	if snapshot.Path != nil {
		t.Fatal("a failed collector must not contribute a part")
	}
	if !collector.Degraded(timings) {
		t.Fatal("failure was not reported as degraded")
	}
}

func TestRunnerWithNoCollectors(t *testing.T) {
	snapshot, timings := collector.Runner{}.Run(context.Background())

	if snapshot.SchemaVersion != 1 {
		t.Errorf("schema version = %d", snapshot.SchemaVersion)
	}
	if len(timings) != 0 {
		t.Errorf("timings = %+v", timings)
	}
	if collector.Degraded(timings) {
		t.Error("empty run reported as degraded")
	}
}
