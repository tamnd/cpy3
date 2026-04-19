/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

import (
	"sync"
	"testing"
)

var initOnce sync.Once

// setupPy initializes the interpreter on the first call (releasing the
// GIL so other goroutines can Acquire it), then pins the calling
// goroutine to its OS thread and acquires the GIL. A cleanup releases
// the GIL when the test ends.
//
// Python 3.12+ aborts the process if a Py_* call runs on a thread
// that does not hold the GIL, and the Go runtime freely migrates
// goroutines across OS threads. Every test that touches the C API
// must therefore pin its goroutine and hold the GIL.
func setupPy(tb testing.TB) {
	tb.Helper()
	initOnce.Do(func() {
		Py_Initialize()
		// Py_Initialize leaves the GIL held on the calling thread.
		// Release it so subsequent PyGILState_Ensure calls from other
		// goroutines succeed.
		releaseGIL()
	})
	release := Acquire()
	tb.Cleanup(release)
}
