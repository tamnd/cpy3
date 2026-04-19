/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

import (
	"os"
	"runtime"
	"testing"
)

// TestMain pins the test-running goroutine to a single OS thread for
// the duration of the process. Python 3.12+ enforces that every Py_*
// call happen on the GIL-holding thread, and the Go runtime freely
// migrates goroutines across OS threads by default. Without this pin
// the legacy suite (which interleaves Py_Initialize / Py_Finalize and
// does not use python3.Acquire) randomly trips the strict-GIL check
// and aborts the process.
//
// Tests that want to opt into the modern, migration-safe model should
// use `defer python3.Acquire()()` at the top of the test. TestMain is
// a safety net for the rest.
func TestMain(m *testing.M) {
	runtime.LockOSThread()
	os.Exit(m.Run())
}
