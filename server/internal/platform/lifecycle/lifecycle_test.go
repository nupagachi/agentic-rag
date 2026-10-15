package lifecycle

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func newTest() *Lifecycle {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return New(log)
}

func TestStopRunsHooksInReverseOrder(t *testing.T) {
	lc := newTest()
	var order []string
	for _, name := range []string{"postgres", "redis", "queue"} {
		lc.OnStop(name, func(context.Context) error { order = append(order, name); return nil })
	}

	if err := lc.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	want := []string{"queue", "redis", "postgres"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestStopCollectsErrorsAndRunsAllHooks(t *testing.T) {
	lc := newTest()
	boom := errors.New("boom")
	ran := false
	lc.OnStop("first", func(context.Context) error { ran = true; return nil })
	lc.OnStop("second", func(context.Context) error { return boom })

	err := lc.Stop(t.Context())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if !ran {
		t.Fatal("hook after failing hook did not run")
	}
}

func TestShutdownCancelsContextAndStopWaitsForTasks(t *testing.T) {
	lc := newTest()
	finished := make(chan struct{})
	lc.Go("worker", func(ctx context.Context) {
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond) // dọn dẹp
		close(finished)
	})
	closedAfterTask := false
	lc.OnStop("db", func(context.Context) error {
		select {
		case <-finished:
			closedAfterTask = true
		default:
		}
		return nil
	})

	if lc.ShuttingDown() {
		t.Fatal("ShuttingDown before Shutdown")
	}
	lc.Shutdown()
	if !lc.ShuttingDown() || lc.Context().Err() == nil {
		t.Fatal("Shutdown did not mark state / cancel context")
	}
	if err := lc.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !closedAfterTask {
		t.Fatal("hook ran before background task finished")
	}
}

func TestStopTimesOutOnStuckTaskButStillRunsHooks(t *testing.T) {
	lc := newTest()
	release := make(chan struct{})
	defer close(release)
	lc.Go("stuck", func(context.Context) { <-release })
	hookRan := false
	lc.OnStop("db", func(context.Context) error { hookRan = true; return nil })

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	err := lc.Stop(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if !hookRan {
		t.Fatal("hook not run after task timeout")
	}
}
