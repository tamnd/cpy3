/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

/*
#include "Python.h"
*/
import "C"
import "runtime"

// Acquire locks the calling goroutine to its OS thread and acquires
// Python's GIL on that thread. It returns a release function that
// drops the GIL and unlocks the thread.
//
// The idiomatic call shape is:
//
//	defer python3.Acquire()()
//
// Python 3.12 made it a hard crash to call any Py_* API from an OS
// thread that does not hold the GIL. Go freely reschedules goroutines
// across OS threads, so every Go call-site that touches the C API must
// pin its goroutine and hold the GIL. Acquire does both.
//
// Acquire composes: calling it again inside a section that already
// holds the GIL increments PyGILState's internal counter, and the
// matching release is a no-op at the CPython layer.
func Acquire() func() {
	runtime.LockOSThread()
	state := C.PyGILState_Ensure()
	return func() {
		C.PyGILState_Release(state)
		runtime.UnlockOSThread()
	}
}

// WithGIL runs fn with the GIL held on the current OS thread. It is a
// convenience wrapper around Acquire for callers who prefer a closure.
func WithGIL(fn func()) {
	defer Acquire()()
	fn()
}

// releaseGIL drops the GIL on the current thread without the paired
// Ensure. It is the counterpart of PyEval_SaveThread, used once at
// interpreter bring-up so that subsequent PyGILState_Ensure calls
// from other OS threads can actually succeed. Leaving the GIL held
// on the init thread deadlocks every other goroutine that tries to
// Acquire.
//
// This is internal plumbing; test helpers and [New] use it. End
// users should rely on [New] or the setup helpers instead.
func releaseGIL() {
	C.PyEval_SaveThread()
}
