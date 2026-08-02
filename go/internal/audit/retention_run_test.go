package audit_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// quietLogger returns a logger that discards output so the sweeper's
// start and per-removal lines do not pollute test output.
func quietLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// TestRunSweepsImmediatelyThenOnEveryTick drives the background loop on
// synthetic time. The contract has two halves that a single Sweep call cannot
// show: the first pass happens before any tick elapses, and each later tick
// runs another pass. A file that expires only after the loop has started
// proves the second half.
func TestRunSweepsImmediatelyThenOnEveryTick(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)

		firstPass := writeRotatedFile(t, dir, "audit-2026-05-01.log")
		secondPass := writeRotatedFile(t, dir, "audit-2026-05-06.log.gz")
		kept := writeRotatedFile(t, dir, "audit-2026-05-18.log.gz")

		// The clock advances between passes. At "now" the cutoff is 2026-05-05,
		// so only the 05-01 file is expired; three days later the cutoff moves
		// to 05-08 and the 05-06 file expires too. That gap is what separates
		// the immediate pass from the tick that follows it.
		// The sweeper reads the clock from its own goroutine, so the instant
		// has to move across goroutines through an atomic rather than a plain
		// variable.
		var clock atomic.Int64

		clock.Store(now.UnixNano())

		sweeper := audit.NewRetentionSweeper(
			dir, 14,
			audit.WithSweepInterval(time.Hour),
			audit.WithSweepLogger(quietLogger()),
			audit.WithSweepClock(func() time.Time {
				return time.Unix(0, clock.Load()).UTC()
			}),
		)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go sweeper.Run(ctx)

		synctest.Wait()

		if _, err := os.Stat(firstPass); !os.IsNotExist(err) {
			t.Errorf("stat %s after the immediate pass = %v, want the file removed",
				filepath.Base(firstPass), err)
		}

		if _, err := os.Stat(secondPass); err != nil {
			t.Errorf("stat %s after the immediate pass = %v, want it still present",
				filepath.Base(secondPass), err)
		}

		clock.Store(now.AddDate(0, 0, 3).UnixNano())

		time.Sleep(time.Hour + time.Minute)
		synctest.Wait()

		if _, err := os.Stat(secondPass); !os.IsNotExist(err) {
			t.Errorf("stat %s after one tick = %v, want the file removed",
				filepath.Base(secondPass), err)
		}

		if _, err := os.Stat(kept); err != nil {
			t.Errorf("stat %s = %v, want a file inside the window kept", filepath.Base(kept), err)
		}

		cancel()
		synctest.Wait()
	})
}

// TestRunStopsWhenTheContextIsCanceled pins the shutdown half of the loop:
// once the context is done the goroutine has to return, otherwise the sweeper
// would outlive the server it was started for.
func TestRunStopsWhenTheContextIsCanceled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		sweeper := audit.NewRetentionSweeper(
			t.TempDir(), 14,
			audit.WithSweepInterval(time.Hour),
			audit.WithSweepLogger(quietLogger()),
		)

		ctx, cancel := context.WithCancel(t.Context())
		stopped := make(chan struct{})

		go func() {
			defer close(stopped)

			sweeper.Run(ctx)
		}()

		synctest.Wait()
		cancel()
		synctest.Wait()

		select {
		case <-stopped:
		default:
			t.Error("Run is still going after its context was canceled, want it returned")
		}
	})
}

// TestRunSurvivesAnUnreadableDirectory keeps a failed pass from killing the
// loop. Sweep reports a directory-level error when the audit directory is
// gone; the loop is supposed to log it and keep ticking rather than exit.
func TestRunSurvivesAnUnreadableDirectory(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		sweeper := audit.NewRetentionSweeper(
			filepath.Join(t.TempDir(), "missing"), 14,
			audit.WithSweepInterval(time.Hour),
			audit.WithSweepLogger(quietLogger()),
		)

		ctx, cancel := context.WithCancel(t.Context())
		stopped := make(chan struct{})

		go func() {
			defer close(stopped)

			sweeper.Run(ctx)
		}()

		synctest.Wait()

		time.Sleep(2*time.Hour + time.Minute)
		synctest.Wait()

		select {
		case <-stopped:
			t.Error("Run returned after a failed sweep, want the loop to keep going")
		default:
		}

		cancel()
		synctest.Wait()
	})
}

// TestSweepOptionsIgnoreUselessValues covers the guard inside each option: a
// zero interval or nil logger would leave the sweeper unusable, so the option
// keeps the constructor's default instead of storing the bad value.
func TestSweepOptionsIgnoreUselessValues(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		sweeper := audit.NewRetentionSweeper(
			t.TempDir(), 14,
			audit.WithSweepInterval(0),
			audit.WithSweepLogger(nil),
			audit.WithSweepClock(nil),
		)

		ctx, cancel := context.WithCancel(t.Context())
		stopped := make(chan struct{})

		go func() {
			defer close(stopped)

			sweeper.Run(ctx)
		}()

		synctest.Wait()

		// The default interval is far longer than an hour, so no tick can have
		// fired yet; the loop being alive is what proves the options left a
		// working sweeper behind rather than a zero-interval ticker panic.
		select {
		case <-stopped:
			t.Error("Run returned immediately, want a sweeper built on the constructor defaults")
		default:
		}

		cancel()
		synctest.Wait()
	})
}

// TestNoopSinkDiscardsEvents pins the discard contract the capture middleware
// relies on when auditing is switched off: Write accepts an event, keeps
// nothing, and never touches the pointer it was handed.
func TestNoopSinkDiscardsEvents(t *testing.T) {
	t.Parallel()

	event := &audit.Event{Tool: "linode_instance_list"}

	audit.NoopSink{}.Write(t.Context(), event)

	if event.Tool != "linode_instance_list" {
		t.Errorf("event.Tool = %q, want the sink to leave the event untouched", event.Tool)
	}
}
