// Package collector reads machine state into a Snapshot.
//
// A collector reads one slice, never interprets it, and never modifies
// anything. Interpretation belongs to rules, which receive a finished
// Snapshot and are not allowed to touch the operating system.
package collector

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DefaultTimeout is the deadline applied to each collector when Runner.Timeout
// is zero. PLAN section 14 requires every collector to finish within 2 s.
const DefaultTimeout = 2 * time.Second

// Collector reads one slice of machine state.
type Collector interface {
	// Name identifies the collector in timings and degraded notices.
	Name() string
	// Collect returns only the fields it owns. Returning an error leaves those
	// fields absent and marks the run degraded.
	Collect(ctx context.Context) (Part, error)
}

// Part is the subset of Snapshot that a single collector is allowed to fill.
// The type is a struct rather than an interface so that adding a collector is
// a compile-time change here, reviewed once.
type Part struct {
	Platform *Platform
	Shell    *Shell
	Path     *PathInfo
	Runtimes []Runtime
	Ports    []Port
	Docker   *Docker
	Git      *Git
}

// Timing records how long one collector took, whether it failed, and why.
type Timing struct {
	Name     string
	Duration time.Duration
	Err      error
}

// Degraded reports whether any collector failed.
func Degraded(timings []Timing) bool {
	for _, t := range timings {
		if t.Err != nil {
			return true
		}
	}
	return false
}

// Runner executes collectors in parallel with a per-collector deadline.
type Runner struct {
	Collectors []Collector
	Timeout    time.Duration
	// Now is injectable for deterministic tests.
	Now func() time.Time
}

type outcome struct {
	index int
	part  Part
	err   error
	start time.Time
	end   time.Time
}

// Run executes every collector, waits for all of them, and returns a Snapshot
// plus one Timing per collector in registration order.
//
// A failed collector never aborts the run: it is recorded in Snapshot.Errors
// and the caller decides the exit code. Results are merged after all
// goroutines finish, so no mutex is needed.
func (r Runner) Run(ctx context.Context) (Snapshot, []Timing) {
	now := r.Now
	if now == nil {
		now = time.Now
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	snapshot := Snapshot{
		SchemaVersion: 1,
		CollectedAt:   now().UTC(),
	}
	if len(r.Collectors) == 0 {
		return snapshot, nil
	}

	results := make(chan outcome, len(r.Collectors))
	for i, c := range r.Collectors {
		go func(index int, c Collector) {
			start := time.Now()
			collectCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// A slow filesystem call may not observe cancellation. Bound the
			// runner's wait as well; the buffered result lets a late collector exit.
			done := make(chan outcome, 1)
			go func() {
				part, err := c.Collect(collectCtx)
				done <- outcome{part: part, err: err}
			}()
			var part Part
			var err error
			select {
			case result := <-done:
				part, err = result.part, result.err
			case <-collectCtx.Done():
				err = collectCtx.Err()
			}
			if err != nil && errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("timeout after %s", timeout)
			}
			results <- outcome{index: index, part: part, err: err, start: start, end: time.Now()}
		}(i, c)
	}

	outcomes := make([]outcome, len(r.Collectors))
	for range r.Collectors {
		result := <-results
		outcomes[result.index] = result
	}

	timings := make([]Timing, 0, len(outcomes))
	for _, result := range outcomes {
		name := r.Collectors[result.index].Name()
		timings = append(timings, Timing{
			Name:     name,
			Duration: result.end.Sub(result.start),
			Err:      result.err,
		})
		if result.err != nil {
			snapshot.Errors = append(snapshot.Errors, CollectorError{
				Collector: name,
				Message:   result.err.Error(),
			})
			continue
		}
		mergePart(&snapshot, result.part)
	}
	return snapshot, timings
}

func mergePart(snapshot *Snapshot, part Part) {
	if part.Runtimes != nil {
		snapshot.Runtimes = part.Runtimes
	}
	if part.Ports != nil {
		snapshot.Ports = part.Ports
	}
	if part.Docker != nil {
		snapshot.Docker = part.Docker
	}
	if part.Git != nil {
		snapshot.Git = part.Git
	}
	if part.Platform != nil {
		snapshot.Platform = part.Platform
	}
	if part.Shell != nil {
		snapshot.Shell = part.Shell
	}
	if part.Path != nil {
		snapshot.Path = part.Path
	}
}
