/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

/*
#include "Python.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// Interp is a handle to a Python interpreter. The zero value is not
// useful; construct one with New or obtain the process-wide default
// via Default.
//
// A program typically needs exactly one Interp. CPython supports
// subinterpreters, but they share process-wide state such as signal
// handlers and atexit callbacks, so most embedders treat the first
// Interp as the authoritative one.
//
// Interp methods acquire the GIL on the calling OS thread before
// invoking the C API, so a caller can use an Interp from any
// goroutine without managing the GIL by hand.
type Interp struct {
	closed bool
}

// Option configures a new Interp. Pass options to New.
type Option func(*interpConfig)

type interpConfig struct {
	programName string
	pythonHome  string
	searchPaths []string
	argv        []string
	parseArgv   bool
	isolated    bool
	stdioEnc    string
	stdioErr    string
}

// WithProgramName sets PyConfig.program_name.
func WithProgramName(name string) Option {
	return func(c *interpConfig) { c.programName = name }
}

// WithPythonHome sets PyConfig.home.
func WithPythonHome(home string) Option {
	return func(c *interpConfig) { c.pythonHome = home }
}

// WithSearchPaths prepends paths to PyConfig.module_search_paths. The
// resulting interpreter sees these entries at the front of sys.path.
func WithSearchPaths(paths ...string) Option {
	return func(c *interpConfig) { c.searchPaths = paths }
}

// WithArgs sets PyConfig.argv. If parse is true, Python consumes
// option flags such as -c or -m from the list, mirroring the old
// PySys_SetArgvEx(updatepath=true) behaviour.
func WithArgs(parse bool, argv ...string) Option {
	return func(c *interpConfig) {
		c.argv = argv
		c.parseArgv = parse
	}
}

// WithStdio sets PyConfig.stdio_encoding and PyConfig.stdio_errors.
func WithStdio(encoding, errors string) Option {
	return func(c *interpConfig) {
		c.stdioEnc = encoding
		c.stdioErr = errors
	}
}

// Isolated configures the new Interp with PyConfig_InitIsolatedConfig:
// no environment variables, no user site, no signal handlers. Use this
// when the host program is the only source of truth for sys.path and
// friends.
func Isolated() Option {
	return func(c *interpConfig) { c.isolated = true }
}

var (
	interpMu      sync.Mutex
	defaultInterp *Interp
)

// New initializes the Python interpreter and returns a handle. Only
// the first call actually brings the interpreter up; subsequent calls
// return an additional handle to the same interpreter. Close on any
// handle tears the interpreter down.
//
// New pins the calling goroutine to an OS thread for the duration of
// interpreter start-up, then releases the GIL so other goroutines can
// Acquire it.
func New(opts ...Option) (*Interp, error) {
	cfg := &interpConfig{parseArgv: false}
	for _, opt := range opts {
		opt(cfg)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if Py_IsInitialized() {
		// Second-or-later New on an already-initialized interpreter.
		// The config cannot be re-applied, so reject options that
		// would otherwise be silently ignored.
		if cfg.programName != "" || cfg.pythonHome != "" ||
			len(cfg.searchPaths) != 0 || len(cfg.argv) != 0 ||
			cfg.stdioEnc != "" || cfg.isolated {
			return nil, fmt.Errorf("python3: interpreter already initialized; options are ignored")
		}
		return &Interp{}, nil
	}

	mode := PyConfigPython
	if cfg.isolated {
		mode = PyConfigIsolated
	}
	pc := NewPyConfig(mode)
	if pc == nil {
		return nil, fmt.Errorf("python3: PyConfig allocation failed")
	}

	if cfg.programName != "" {
		if s := pc.SetProgramName(cfg.programName); !s.IsOk() {
			pc.Clear()
			return nil, s.Err()
		}
	}
	if cfg.pythonHome != "" {
		if s := pc.SetPythonHome(cfg.pythonHome); !s.IsOk() {
			pc.Clear()
			return nil, s.Err()
		}
	}
	if cfg.stdioEnc != "" || cfg.stdioErr != "" {
		if s := pc.SetStdioEncoding(cfg.stdioEnc, cfg.stdioErr); !s.IsOk() {
			pc.Clear()
			return nil, s.Err()
		}
	}
	if len(cfg.argv) != 0 {
		if s := pc.SetArgv(cfg.argv, cfg.parseArgv); !s.IsOk() {
			pc.Clear()
			return nil, s.Err()
		}
	}
	if len(cfg.searchPaths) != 0 {
		if s := pc.SetModuleSearchPaths(cfg.searchPaths); !s.IsOk() {
			pc.Clear()
			return nil, s.Err()
		}
	}

	status := Py_InitializeFromConfig(pc)
	pc.Clear()
	if !status.IsOk() {
		return nil, status.Err()
	}

	// Release the GIL so other goroutines can Acquire it.
	C.PyEval_SaveThread()

	return &Interp{}, nil
}

// Default returns the process-wide interpreter, initializing it with
// default options on first use. If initialization fails, Default
// panics; callers who want to surface the error should use New.
func Default() *Interp {
	interpMu.Lock()
	defer interpMu.Unlock()
	if defaultInterp != nil {
		return defaultInterp
	}
	i, err := New()
	if err != nil {
		panic(fmt.Sprintf("python3.Default: %v", err))
	}
	defaultInterp = i
	return i
}

// Close finalizes the interpreter. It is safe to call Close more than
// once; subsequent calls are no-ops.
func (p *Interp) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true

	release := Acquire()
	defer release()
	if code := Py_FinalizeEx(); code != 0 {
		return fmt.Errorf("python3: Py_FinalizeEx returned %d", code)
	}
	return nil
}

// Acquire acquires the GIL on the current OS thread and locks the
// calling goroutine to that thread. See the package-level Acquire for
// details.
func (p *Interp) Acquire() func() {
	return Acquire()
}

// Run executes a Python source fragment in the __main__ module's
// namespace and returns any uncaught exception as an error.
func (p *Interp) Run(code string) error {
	defer Acquire()()

	ccode := C.CString(code)
	defer C.free(unsafe.Pointer(ccode))

	if C.PyRun_SimpleString(ccode) != 0 {
		return errorFromPython()
	}
	return nil
}

// Import imports the named module and returns a handle. The caller
// owns the returned Object and must Close it when done.
func (p *Interp) Import(name string) (*Object, error) {
	defer Acquire()()

	mod := PyImport_ImportModule(name)
	if mod == nil {
		return nil, errorFromPython()
	}
	return newObject(mod), nil
}

// Eval evaluates a Python expression in the __main__ namespace and
// returns the result. The caller owns the returned Object.
func (p *Interp) Eval(expr string) (*Object, error) {
	defer Acquire()()

	cexpr := C.CString(expr)
	defer C.free(unsafe.Pointer(cexpr))

	main := C.CString("__main__")
	defer C.free(unsafe.Pointer(main))
	mod := C.PyImport_AddModule(main)
	if mod == nil {
		return nil, errorFromPython()
	}
	globals := C.PyModule_GetDict(mod)

	result := C.PyRun_String(cexpr, C.Py_eval_input, globals, globals)
	if result == nil {
		return nil, errorFromPython()
	}
	return newObject((*PyObject)(result)), nil
}
