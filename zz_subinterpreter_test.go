/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

// This file is named with a zz_ prefix so it sorts after the rest of
// the package's test files. CPython's PyGILState_*() API is explicitly
// documented as unsuitable for subinterpreters (see the C-API docs on
// PyGILState_Check): once a subinterpreter has been created and
// destroyed in a process, PyGILState_Check on the main interpreter
// can return stale results for the remainder of the test binary.
// Pre-existing tests (notably thread_test.go:TestThreadSaveRestore)
// assert against PyGILState_Check, so the subinterpreter tests run
// last to keep those assertions valid.

package python3

import (
	"sync"
	"testing"
)

// runInSub acquires the main GIL on a fresh goroutine, creates a
// subinterpreter with the given config, invokes fn, and tears the sub
// down. Any error from NewSubInterpreter, Close, or fn is returned.
//
// Factoring this into a helper keeps the bookkeeping (Acquire pair,
// lifetime, Close-on-error) out of each test body.
func runInSub(cfg SubInterpreterConfig, fn func(*SubInterpreter) error) error {
	done := make(chan error, 1)
	go func() {
		release := Acquire()
		defer release()

		sub, err := NewSubInterpreter(cfg)
		if err != nil {
			done <- err
			return
		}
		runErr := fn(sub)
		closeErr := sub.Close()
		if runErr != nil {
			done <- runErr
			return
		}
		done <- closeErr
	}()
	return <-done
}

// TestSubInterpreter_OwnGIL_Lifecycle creates a subinterpreter with its
// own GIL, runs a trivial statement inside it, and closes it. Getting
// through the open-close cycle without aborting the process is the
// first sanity check for Py_NewInterpreterFromConfig.
func TestSubInterpreter_OwnGIL_Lifecycle(t *testing.T) {
	_ = Default() // bring the main interpreter up

	err := runInSub(DefaultSubInterpreterConfig(), func(sub *SubInterpreter) error {
		return sub.Run("x = 6 * 7")
	})
	if err != nil {
		t.Fatalf("subinterpreter lifecycle: %v", err)
	}
}

// TestSubInterpreter_Isolation verifies that state set in one
// subinterpreter is not visible in another. Each sub has its own
// __main__ namespace.
func TestSubInterpreter_Isolation(t *testing.T) {
	_ = Default()

	if err := runInSub(DefaultSubInterpreterConfig(), func(sub *SubInterpreter) error {
		return sub.Run("only_in_a = 1")
	}); err != nil {
		t.Fatalf("sub a: %v", err)
	}

	err := runInSub(DefaultSubInterpreterConfig(), func(sub *SubInterpreter) error {
		// only_in_a must not leak into this fresh sub.
		if err := sub.Run("only_in_a"); err == nil {
			return pyTestError("expected NameError for only_in_a in fresh sub")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("sub b: %v", err)
	}
}

// TestSubInterpreter_ConcurrentOwnGIL launches two subinterpreters on
// separate goroutines and runs compute-bound Python code in parallel.
// Each subinterpreter has its own GIL, so the work can interleave on
// real OS threads rather than serializing behind a single lock. The
// test only asserts correctness; PEP 684 parallelism is observed in
// the Docker-based benchmarks, not here.
func TestSubInterpreter_ConcurrentOwnGIL(t *testing.T) {
	_ = Default()

	const n = 2
	var wg sync.WaitGroup
	errs := make(chan error, n)
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			errs <- runInSub(DefaultSubInterpreterConfig(), func(sub *SubInterpreter) error {
				// A tight loop that exercises bytecode, integer math,
				// and the GC, all of which are per-interpreter under
				// PEP 684.
				return sub.Run(`
total = 0
for i in range(1000):
    total += i * i
assert total == sum(i * i for i in range(1000))
`)
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent sub: %v", err)
		}
	}
}

// pyTestError is a tiny error type so the test file does not need an
// "errors" import for one-shot messages.
type pyTestError string

func (e pyTestError) Error() string { return string(e) }
