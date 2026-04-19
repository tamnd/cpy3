/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

/*
#include <stdlib.h>
#include <wchar.h>
#include "Python.h"

// PyConfig is not a plain struct in cgo's eyes: its layout is visible
// but Go cannot take the address of the trailing flexible array. Wrap
// allocation and the string setters in small C helpers so the Go side
// only talks to pointers.

// Use plain malloc/free for the struct itself. PyMem_RawMalloc would
// be tagged by Python's debug allocator only if the interpreter is up
// when we allocate, but NewPyConfig is meant to be called before
// Py_InitializeFromConfig — and PyMem_RawFree in debug mode then
// refuses to free an untagged block.
static PyConfig* _go_PyConfig_New(int isolated) {
    PyConfig *c = (PyConfig*)malloc(sizeof(PyConfig));
    if (c == NULL) {
        return NULL;
    }
    if (isolated) {
        PyConfig_InitIsolatedConfig(c);
    } else {
        PyConfig_InitPythonConfig(c);
    }
    return c;
}

static void _go_PyConfig_Free(PyConfig *c) {
    if (c == NULL) {
        return;
    }
    PyConfig_Clear(c);
    free(c);
}

static int _go_PyStatus_Exception(PyStatus s) {
    return PyStatus_Exception(s);
}

static int _go_PyStatus_IsExit(PyStatus s) {
    return PyStatus_IsExit(s);
}

static int _go_PyStatus_IsError(PyStatus s) {
    return PyStatus_IsError(s);
}

static int _go_PyStatus_ExitCode(PyStatus s) {
    return s.exitcode;
}

static const char* _go_PyStatus_ErrMsg(PyStatus s) {
    return s.err_msg;
}

static const char* _go_PyStatus_Func(PyStatus s) {
    return s.func;
}

static PyStatus _go_PyConfig_SetString(PyConfig *c, wchar_t **field, const char *value) {
    return PyConfig_SetBytesString(c, field, value);
}

static PyStatus _go_PyConfig_SetProgramName(PyConfig *c, const char *value) {
    return PyConfig_SetBytesString(c, &c->program_name, value);
}

static PyStatus _go_PyConfig_SetPythonHome(PyConfig *c, const char *value) {
    return PyConfig_SetBytesString(c, &c->home, value);
}

static PyStatus _go_PyConfig_SetStdioEncoding(PyConfig *c, const char *enc, const char *errors) {
    PyStatus s = PyConfig_SetBytesString(c, &c->stdio_encoding, enc);
    if (PyStatus_Exception(s)) {
        return s;
    }
    return PyConfig_SetBytesString(c, &c->stdio_errors, errors);
}

static PyStatus _go_PyConfig_SetArgv(PyConfig *c, int argc, char **argv) {
    return PyConfig_SetBytesArgv(c, argc, argv);
}

static PyStatus _go_PyConfig_SetArgvWithParse(PyConfig *c, int argc, char **argv, int parse) {
    c->parse_argv = parse;
    return PyConfig_SetBytesArgv(c, argc, argv);
}

static PyStatus _go_PyConfig_SetModuleSearchPaths(PyConfig *c, int count, char **paths) {
    // Free the fresh-init'd list by hand; PyWideStringList_Clear is
    // not part of the public API.
    for (Py_ssize_t i = 0; i < c->module_search_paths.length; i++) {
        PyMem_RawFree(c->module_search_paths.items[i]);
    }
    PyMem_RawFree(c->module_search_paths.items);
    c->module_search_paths.length = 0;
    c->module_search_paths.items = NULL;
    c->module_search_paths_set = 1;
    for (int i = 0; i < count; i++) {
        wchar_t *w = Py_DecodeLocale(paths[i], NULL);
        if (w == NULL) {
            return PyStatus_NoMemory();
        }
        PyStatus s = PyWideStringList_Append(&c->module_search_paths, w);
        PyMem_RawFree(w);
        if (PyStatus_Exception(s)) {
            return s;
        }
    }
    return PyStatus_Ok();
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// PyConfig wraps PyConfig from the CPython initialization API.
// See https://docs.python.org/3/c-api/init_config.html#c.PyConfig.
//
// A PyConfig must be freed with Clear once the caller is done with it,
// regardless of whether Py_InitializeFromConfig succeeded.
type PyConfig struct {
	c *C.PyConfig
}

// PyConfigMode selects between the two init presets CPython ships.
type PyConfigMode int

const (
	// PyConfigPython mirrors Python's default command-line behaviour:
	// it reads environment variables, honours -E/-I, and so on. Use
	// this when embedding a regular interpreter.
	PyConfigPython PyConfigMode = 0

	// PyConfigIsolated turns off every implicit configuration source:
	// no environment variables, no user site, no signal handlers. Use
	// this when the embedding host is the only source of truth.
	PyConfigIsolated PyConfigMode = 1
)

// NewPyConfig allocates and initializes a PyConfig in the given mode.
func NewPyConfig(mode PyConfigMode) *PyConfig {
	c := C._go_PyConfig_New(C.int(mode))
	if c == nil {
		return nil
	}
	return &PyConfig{c: c}
}

// Clear releases every string owned by the PyConfig and the struct
// itself. Safe to call more than once.
func (cfg *PyConfig) Clear() {
	if cfg == nil || cfg.c == nil {
		return
	}
	C._go_PyConfig_Free(cfg.c)
	cfg.c = nil
}

// SetProgramName sets PyConfig.program_name. Equivalent to the
// pre-3.13 Py_SetProgramName.
func (cfg *PyConfig) SetProgramName(name string) *PyStatus {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return fromCStatus(C._go_PyConfig_SetProgramName(cfg.c, cname))
}

// SetPythonHome sets PyConfig.home.
func (cfg *PyConfig) SetPythonHome(home string) *PyStatus {
	chome := C.CString(home)
	defer C.free(unsafe.Pointer(chome))
	return fromCStatus(C._go_PyConfig_SetPythonHome(cfg.c, chome))
}

// SetStdioEncoding sets PyConfig.stdio_encoding and
// PyConfig.stdio_errors together, as the pair is almost always changed
// at once.
func (cfg *PyConfig) SetStdioEncoding(encoding, errors string) *PyStatus {
	cenc := C.CString(encoding)
	defer C.free(unsafe.Pointer(cenc))
	cerr := C.CString(errors)
	defer C.free(unsafe.Pointer(cerr))
	return fromCStatus(C._go_PyConfig_SetStdioEncoding(cfg.c, cenc, cerr))
}

// SetArgv sets PyConfig.argv. If parseArgv is true, Python will also
// consume options such as -c or -m from the argument list, matching
// the old PySys_SetArgvEx(updatepath=true) behaviour.
func (cfg *PyConfig) SetArgv(args []string, parseArgv bool) *PyStatus {
	argc, argv, free := cStringSlice(args)
	defer free()
	parse := C.int(0)
	if parseArgv {
		parse = 1
	}
	return fromCStatus(C._go_PyConfig_SetArgvWithParse(cfg.c, argc, argv, parse))
}

// SetModuleSearchPaths sets PyConfig.module_search_paths and marks it
// as user-provided. Replaces the pre-3.13 Py_SetPath.
func (cfg *PyConfig) SetModuleSearchPaths(paths []string) *PyStatus {
	argc, argv, free := cStringSlice(paths)
	defer free()
	return fromCStatus(C._go_PyConfig_SetModuleSearchPaths(cfg.c, argc, argv))
}

// ProgramName reads back PyConfig.program_name.
func (cfg *PyConfig) ProgramName() string {
	return wcharToString(cfg.c.program_name)
}

// PythonHome reads back PyConfig.home.
func (cfg *PyConfig) PythonHome() string {
	return wcharToString(cfg.c.home)
}

// Py_InitializeFromConfig initializes the interpreter from cfg. The
// caller keeps ownership of cfg and is responsible for calling
// Clear once it is done with the struct.
//
// See https://docs.python.org/3/c-api/init_config.html#c.Py_InitializeFromConfig.
func Py_InitializeFromConfig(cfg *PyConfig) *PyStatus {
	if cfg == nil || cfg.c == nil {
		return &PyStatus{err: "nil PyConfig"}
	}
	return fromCStatus(C.Py_InitializeFromConfig(cfg.c))
}

// Py_RunMain runs the main interpreter loop, as the python3 binary
// would, then finalizes the interpreter. Returns the exit code.
//
// See https://docs.python.org/3/c-api/init_config.html#c.Py_RunMain.
func Py_RunMain() int {
	return int(C.Py_RunMain())
}

// PyStatus mirrors PyStatus from init_config.h.
type PyStatus struct {
	exit    bool
	exit_   int
	errored bool
	err     string
	fn      string
}

// IsOk reports that the status represents success.
func (s *PyStatus) IsOk() bool {
	return s == nil || (!s.errored && !s.exit)
}

// IsError reports that the status represents a non-zero error.
func (s *PyStatus) IsError() bool {
	return s != nil && s.errored
}

// IsExit reports that the status represents a clean exit request.
func (s *PyStatus) IsExit() bool {
	return s != nil && s.exit
}

// ExitCode returns the exit code, or zero on success.
func (s *PyStatus) ExitCode() int {
	if s == nil {
		return 0
	}
	return s.exit_
}

// Err returns a Go error if the status represents a failure.
func (s *PyStatus) Err() error {
	if s.IsOk() {
		return nil
	}
	if s.exit {
		return fmt.Errorf("python: exit %d", s.exit_)
	}
	if s.fn != "" {
		return fmt.Errorf("python: %s: %s", s.fn, s.err)
	}
	return fmt.Errorf("python: %s", s.err)
}

func fromCStatus(s C.PyStatus) *PyStatus {
	if C._go_PyStatus_Exception(s) == 0 {
		return &PyStatus{}
	}
	out := &PyStatus{}
	if C._go_PyStatus_IsExit(s) != 0 {
		out.exit = true
		out.exit_ = int(C._go_PyStatus_ExitCode(s))
		return out
	}
	out.errored = true
	if msg := C._go_PyStatus_ErrMsg(s); msg != nil {
		out.err = C.GoString(msg)
	}
	if fn := C._go_PyStatus_Func(s); fn != nil {
		out.fn = C.GoString(fn)
	}
	return out
}

// cStringSlice allocates a NULL-free argv-style slice for C.
// The returned free func releases both the pointer array and every
// string it holds.
func cStringSlice(xs []string) (C.int, **C.char, func()) {
	if len(xs) == 0 {
		return 0, nil, func() {}
	}
	arr := C.malloc(C.size_t(len(xs)) * C.size_t(unsafe.Sizeof(uintptr(0))))
	slice := unsafe.Slice((**C.char)(arr), len(xs))
	for i, s := range xs {
		slice[i] = C.CString(s)
	}
	free := func() {
		for i := range slice {
			C.free(unsafe.Pointer(slice[i]))
		}
		C.free(arr)
	}
	return C.int(len(xs)), (**C.char)(arr), free
}

func wcharToString(w *C.wchar_t) string {
	if w == nil {
		return ""
	}
	// Use libc wcstombs: Py_EncodeLocale requires Py_Initialize, but
	// we want to read PyConfig fields before the interpreter is up.
	n := C.wcstombs(nil, w, 0)
	if n == C.size_t(^uintptr(0)) {
		return ""
	}
	buf := C.malloc(n + 1)
	defer C.free(buf)
	C.wcstombs((*C.char)(buf), w, n+1)
	return C.GoStringN((*C.char)(buf), C.int(n))
}
