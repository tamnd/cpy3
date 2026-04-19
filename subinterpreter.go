/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

/*
#include "Python.h"

// The PyInterpreterConfig C struct has seven int fields and is
// initialized via macros (_PyInterpreterConfig_INIT,
// _PyInterpreterConfig_LEGACY_INIT). cgo cannot use those macros
// directly, so we mirror them as small C helpers.

static PyInterpreterConfig _cpy3_interp_config_default(void) {
    PyInterpreterConfig cfg = {
        .use_main_obmalloc = 0,
        .allow_fork = 0,
        .allow_exec = 0,
        .allow_threads = 1,
        .allow_daemon_threads = 0,
        .check_multi_interp_extensions = 1,
        .gil = PyInterpreterConfig_OWN_GIL,
    };
    return cfg;
}

static PyInterpreterConfig _cpy3_interp_config_legacy(void) {
    PyInterpreterConfig cfg = {
        .use_main_obmalloc = 1,
        .allow_fork = 1,
        .allow_exec = 1,
        .allow_threads = 1,
        .allow_daemon_threads = 1,
        .check_multi_interp_extensions = 0,
        .gil = PyInterpreterConfig_SHARED_GIL,
    };
    return cfg;
}

static PyStatus _cpy3_new_interp(PyThreadState **tstate, PyInterpreterConfig *cfg) {
    return Py_NewInterpreterFromConfig(tstate, cfg);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// GILMode selects how a subinterpreter relates to the main
// interpreter's GIL. These values mirror the three documented presets
// from PEP 684.
type GILMode int

const (
	// GILDefault uses PyInterpreterConfig_DEFAULT_GIL, letting CPython
	// pick between shared and own-GIL based on the other options.
	GILDefault GILMode = C.PyInterpreterConfig_DEFAULT_GIL

	// GILShared matches the process-wide main GIL. This is the
	// behavior of Py_NewInterpreter prior to PEP 684.
	GILShared GILMode = C.PyInterpreterConfig_SHARED_GIL

	// GILOwn gives the subinterpreter its own GIL. Two subinterpreters
	// with GILOwn can run Python code concurrently on distinct OS
	// threads. This is the mode PEP 684 was designed for.
	GILOwn GILMode = C.PyInterpreterConfig_OWN_GIL
)

// SubInterpreterConfig mirrors PyInterpreterConfig from CPython 3.12+.
// All zero values are valid and match the PEP 684 default config
// (own GIL, no fork / exec, threads allowed, no daemon threads,
// extension multi-interp check enabled).
type SubInterpreterConfig struct {
	// UseMainObmalloc shares the main interpreter's object allocator
	// arena. Must be false when GIL is GILOwn.
	UseMainObmalloc bool
	AllowFork       bool
	AllowExec       bool
	AllowThreads    bool
	AllowDaemon     bool

	// CheckMultiInterpExtensions rejects single-phase-init extension
	// modules that have not declared subinterpreter support. The
	// free-threaded build forces this to true regardless.
	CheckMultiInterpExtensions bool

	// GIL selects the GIL mode. Zero value is GILDefault.
	GIL GILMode
}

// DefaultSubInterpreterConfig returns the "modern" preset: own GIL,
// no fork / exec, threads allowed, daemon threads forbidden, extension
// compat check on. This is the configuration PEP 684 designed for.
func DefaultSubInterpreterConfig() SubInterpreterConfig {
	return SubInterpreterConfig{
		AllowThreads:               true,
		CheckMultiInterpExtensions: true,
		GIL:                        GILOwn,
	}
}

// LegacySubInterpreterConfig returns the pre-PEP-684 preset: shared
// GIL, main obmalloc, fork / exec / daemon threads allowed. Matches
// the behavior of the old Py_NewInterpreter.
func LegacySubInterpreterConfig() SubInterpreterConfig {
	cfg := SubInterpreterConfig{
		UseMainObmalloc: true,
		AllowFork:       true,
		AllowExec:       true,
		AllowThreads:    true,
		AllowDaemon:     true,
		GIL:             GILShared,
	}
	return cfg
}

// SubInterpreter is a handle to a CPython subinterpreter created via
// Py_NewInterpreterFromConfig. The zero value is not useful; construct
// one with NewSubInterpreter.
//
// A SubInterpreter owns a thread state on the OS thread that created
// it. Callers must not use it from any other goroutine; each
// SubInterpreter is pinned to its creating goroutine for its entire
// lifetime.
type SubInterpreter struct {
	tstate     *C.PyThreadState
	mainTstate *C.PyThreadState
	closed     bool
}

// NewSubInterpreter brings up a subinterpreter using the given config.
//
// Preconditions:
//   - The main interpreter must be initialized (via Default or New).
//   - The caller must hold the main interpreter's GIL on the current
//     OS thread, for example by calling Acquire first.
//
// On return, the caller's OS thread holds the subinterpreter's GIL
// (the main's is suspended). Use Run to execute Python code in the
// sub, and Close to tear it down and restore the main's GIL. Close
// must run on the same goroutine that created the sub.
//
// CPython's PyGILState_*() API is explicitly documented as unsuitable
// for subinterpreters, so this function uses raw PyThreadState
// management. Call Acquire (which uses PyGILState under the hood) on
// a plain goroutine *before* NewSubInterpreter, and pair it with
// Close + the Acquire release in reverse order.
func NewSubInterpreter(cfg SubInterpreterConfig) (*SubInterpreter, error) {
	mainTstate := C.PyThreadState_Get()

	var cc C.PyInterpreterConfig
	if cfg.GIL == GILShared && !cfg.UseMainObmalloc {
		// Legacy preset: mirror the _PyInterpreterConfig_LEGACY_INIT
		// defaults but let callers toggle individual flags.
		cc = C._cpy3_interp_config_legacy()
	} else {
		cc = C._cpy3_interp_config_default()
	}
	cc.use_main_obmalloc = boolToCInt(cfg.UseMainObmalloc)
	cc.allow_fork = boolToCInt(cfg.AllowFork)
	cc.allow_exec = boolToCInt(cfg.AllowExec)
	cc.allow_threads = boolToCInt(cfg.AllowThreads)
	cc.allow_daemon_threads = boolToCInt(cfg.AllowDaemon)
	cc.check_multi_interp_extensions = boolToCInt(cfg.CheckMultiInterpExtensions)
	cc.gil = C.int(cfg.GIL)

	var tstate *C.PyThreadState
	status := C._cpy3_new_interp(&tstate, &cc)
	if C.PyStatus_IsError(status) != 0 {
		// The main's tstate is still current on failure; return the
		// caller's world untouched.
		msg := C.GoString(status.err_msg)
		if msg == "" {
			msg = "python3: Py_NewInterpreterFromConfig failed"
		}
		return nil, errors.New(msg)
	}

	return &SubInterpreter{
		tstate:     tstate,
		mainTstate: mainTstate,
	}, nil
}

// Close ends the subinterpreter and restores the main interpreter's
// thread state and GIL on the current OS thread. After Close the
// caller is back in the pre-NewSubInterpreter state and can drop the
// main GIL via its Acquire release.
//
// Calling Close twice is safe; the second call is a no-op. Close must
// run on the same goroutine that called NewSubInterpreter.
func (s *SubInterpreter) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true

	// Py_EndInterpreter requires its argument to be the current thread
	// state. Run's paired Swap leaves it current; Swap again to be
	// defensive in case a caller interleaved other C-API work.
	C.PyThreadState_Swap(s.tstate)
	C.Py_EndInterpreter(s.tstate)

	// Swap the main tstate back in. This also reacquires the main
	// interpreter's GIL on this thread, so the caller's outer Acquire
	// sees a consistent world when it releases.
	C.PyEval_RestoreThread(s.mainTstate)
	return nil
}

// Run executes a Python statement block inside the subinterpreter's
// __main__ namespace. The subinterpreter's GIL must be held; on a
// SubInterpreter created with NewSubInterpreter it already is.
func (s *SubInterpreter) Run(code string) error {
	if s == nil || s.closed {
		return errors.New("python3: subinterpreter closed")
	}
	prev := C.PyThreadState_Swap(s.tstate)
	defer C.PyThreadState_Swap(prev)

	ccode := C.CString(code)
	defer C.free(unsafe.Pointer(ccode))
	if C.PyRun_SimpleString(ccode) != 0 {
		return errorFromPython()
	}
	return nil
}

func boolToCInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
