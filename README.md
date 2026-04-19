# cpy3

Go bindings for the CPython 3.14 C API.

This is a maintained fork of [`go-python/cpy3`](https://github.com/go-python/cpy3), which stopped receiving updates in 2022 and no longer builds against a modern interpreter. The fork does two things: it fixes the build against CPython 3.14, and it ships an idiomatic Go API on top of the existing thin C wrappers so that most embedders never have to touch `PyObject`, refcounts, or `PyGILState_Ensure` directly.

Both layers live in the same `python3` package. The thin layer is still there when you need a specific `Py*` function; the high-level layer is there when you want to write normal Go.

## Requirements

* Go 1.26 or newer.
* CPython 3.14.0 or newer. Alphas, betas, and release candidates are not supported because the `PyConfig` struct layout churned through the 3.14 prereleases.
* `pkg-config` or `pkgconf` able to resolve `python-3.14-embed`. On macOS this comes with Homebrew `python@3.14`; on Debian and Ubuntu it is in the `python3.14-dev` package; on Red Hat derivatives it is `python3.14-devel`.

The package hard-codes `#cgo pkg-config: python-3.14-embed`. The `-embed` variant is important: programs that link `libpython` (embedders) need the embed pkg-config file, not the plain `python-3.14` one used by extension modules.

## Install

```
go get github.com/go-python/cpy3
```

The import path is `github.com/go-python/cpy3`, kept unchanged from upstream so existing code migrates by bumping the module version.

## Quick start

```go
package main

import (
    "fmt"

    python3 "github.com/go-python/cpy3"
)

func main() {
    p := python3.Default()

    if err := p.Run("x = 6 * 7"); err != nil {
        panic(err)
    }

    v, err := p.Eval("x")
    if err != nil {
        panic(err)
    }
    defer python3.Acquire()()
    defer v.Close()

    got, _ := python3.ToGo[int](v)
    fmt.Println(got) // 42
}
```

That program covers the three things you will do most of the time: bring the interpreter up (`Default`), run or evaluate some Python (`Run`, `Eval`), and pull a value back into Go (`ToGo`). The rest of this document walks through the pieces.

## Why the GIL matters in Go

Python 3.12 made every call to a `Py_*` function from a thread that does not hold the GIL a hard process abort. It was undefined behaviour before; it is an `abort()` now.

Go runs goroutines on a pool of OS threads and migrates them freely. A goroutine that calls into cgo can land on a different OS thread on every call. If that thread never acquired Python's GIL, the next `Py_*` call kills the process.

The canonical fix in C embedders is `PyGILState_Ensure` + `PyGILState_Release` around every call site. In Go the call site also needs to be pinned to an OS thread for the duration, because otherwise the goroutine can be rescheduled between the Ensure and the next C call. `cpy3` wraps both steps in one helper:

```go
defer python3.Acquire()()
```

`Acquire` pins the calling goroutine with `runtime.LockOSThread`, calls `PyGILState_Ensure`, and returns a closure. The closure calls `PyGILState_Release` and `runtime.UnlockOSThread`. The double parentheses are not a typo: the outer call is the defer, the inner call returns the release function.

Nested `Acquire` is safe. `PyGILState_Ensure` increments an internal counter, and the matching Release decrements it. You can call `Acquire` inside a section that already holds the GIL without deadlocking.

`WithGIL(fn func())` is a small closure-style variant for callers who prefer that shape.

### When you need to call Acquire yourself

`Interp.Run`, `Interp.Import`, and `Interp.Eval` acquire the GIL internally. After one of them returns, the GIL is released again. If you then call any `Object` method, read an attribute, call `FromGo` or `ToGo`, or touch the thin `Py*` layer, you have to hold the GIL. The typical shape is:

```go
v, err := p.Eval("some_expression")
if err != nil {
    return err
}
defer python3.Acquire()()
defer v.Close()
// ... use v ...
```

`Object` methods do not acquire the GIL themselves because real call sites batch many operations per Acquire. Acquiring per-call would make method chains and loops unnecessarily expensive.

## The Interp type

`Interp` is a handle to the CPython interpreter.

```go
type Interp struct { /* unexported */ }

func New(opts ...Option) (*Interp, error)
func Default() *Interp

func (*Interp) Close() error
func (*Interp) Acquire() func()
func (*Interp) Run(code string) error
func (*Interp) Import(name string) (*Object, error)
func (*Interp) Eval(expr string) (*Object, error)
```

A program normally needs one. CPython supports subinterpreters, but they share process-wide state like signal handlers and atexit callbacks, so most embedders treat the first `Interp` as authoritative.

`New` accepts functional options:

| Option                                      | Maps to                                      |
| ------------------------------------------- | -------------------------------------------- |
| `WithProgramName(name)`                     | `PyConfig.program_name`                      |
| `WithPythonHome(home)`                      | `PyConfig.home`                              |
| `WithSearchPaths(paths...)`                 | `PyConfig.module_search_paths`               |
| `WithArgs(parse bool, argv...)`             | `PyConfig.argv` (with option parsing if true)|
| `WithStdio(encoding, errors)`               | `PyConfig.stdio_encoding`, `stdio_errors`    |
| `Isolated()`                                | `PyConfig_InitIsolatedConfig`                |

Without `Isolated`, `New` starts from `PyConfig_InitPythonConfig`, which reads environment variables and `sys.path` site packages the way the `python3` binary does. `Isolated` gives you the reverse: no environment, no user site, no signal handlers. Use it when the host program is the only source of truth for `sys.path`.

`New` pins its goroutine to an OS thread with `runtime.LockOSThread` for the duration of interpreter startup, then releases Python's GIL via `PyEval_SaveThread` so other goroutines can `Acquire` it. After `New` returns, the calling goroutine holds no GIL.

The first call to `New` actually initializes CPython. Subsequent calls to `New` against an already-initialized interpreter return an additional handle to the same interpreter. If you pass options on a second call, `New` returns an error rather than silently ignoring them (the config cannot be re-applied after initialization).

`Default()` is a lazy process-wide singleton protected by `sync.Mutex`. It panics on first-call initialization failure; use `New` if you need the error.

`Run` executes a statement block in `__main__`'s namespace and returns any uncaught exception as a Go error. `Eval` evaluates an expression in the same namespace and returns the result as an owning `*Object`. `Import` is the obvious wrapper over `PyImport_ImportModule`.

`Close` calls `Py_FinalizeEx`. It is safe to call more than once. In practice you rarely want to finalize before the program exits, because the lifecycle is not fully reversible: some C extensions cannot be cleanly unloaded.

## Objects

`Object` is a typed reference to a Python object that owns a reference count.

```go
type Object /* underlying type is PyObject */

func (*Object) Close() error           // implements io.Closer, calls Py_DECREF
func (*Object) String() string         // implements fmt.Stringer, returns str(o)
func (*Object) Repr() string
func (*Object) Type() string
func (*Object) IncRef() *Object
func (*Object) Raw() *PyObject

func (*Object) GetAttr(name string) (*Object, error)
func (*Object) SetAttr(name string, value *Object) error
func (*Object) HasAttr(name string) bool

func (*Object) Call(args ...*Object) (*Object, error)
func (*Object) CallMethod(name string, args ...*Object) (*Object, error)

func (*Object) Len() int
```

`Object` is a conversion type over `PyObject`: `(*Object)(p)` and `(*PyObject)(o)` round-trip. The thin layer and the high-level layer alias the same pointer, so dropping between them is free.

`Close` releases one reference. Always pair it with the method that gave you the `Object`: `Eval`, `Import`, `Call`, `GetAttr`, `FromGo`, and `IncRef` all return owning references. Close on a nil receiver is a no-op.

`IncRef` returns a new owning handle to the same underlying object. Use it when handing the value to a caller that will `Close` it while the original handle also wants to `Close` it.

`Call` takes care of the tuple dance: it allocates a `PyTuple`, `IncRef`s each argument (because `PyTuple_SetItem` steals references), sets them, and invokes `PyObject_Call`. If any step fails the intermediate tuple and arguments are released correctly. `CallMethod` is a convenience around `GetAttr` + `Call`.

`Len` returns -1 on error. Use `CheckError` after it if you need the exception.

`Type()` returns the fully qualified name, for example `builtins.list` or `datetime.datetime`. The thin-layer equivalent is `PyObject_Type` + attribute walking, which this method wraps.

## Errors

Python exceptions surface as a typed `*Error`.

```go
type Error struct {
    Type    string // e.g. "builtins.TypeError"
    Message string // str(exception)
    Cause   *Error // __cause__ or __context__, if any
}

func (*Error) Error() string
func (*Error) Unwrap() error

func CheckError() error
func IsPyException(err error) bool
```

Every `Interp` and `Object` method that can fail returns one. The construction path is:

1. `PyErr_GetRaisedException` pulls the exception out of the interpreter (the CPython 3.12+ single-object API; the fork does not use the older `PyErr_Fetch` / `PyErr_Restore` / `PyErr_NormalizeException` triple for new code).
2. The error indicator is cleared, so the caller can raise another exception without interference.
3. `Type` is `__module__.__qualname__` of the exception class, with `builtins.` stripped for the common case.
4. `Message` is `str(exception)`.
5. `Cause` is populated by walking `__cause__` first (PEP 3134 explicit chaining with `raise X from Y`), then falling back to `__context__` (implicit chaining).

Because `Unwrap` is implemented, `errors.Is` and `errors.As` walk the whole chain:

```go
_, err := p.Eval("1/0")
if err != nil {
    var pyErr *python3.Error
    if errors.As(err, &pyErr) {
        fmt.Println(pyErr.Type)    // "builtins.ZeroDivisionError"
        fmt.Println(pyErr.Message) // "division by zero"
    }
}
```

`CheckError` is the low-level primitive: it calls `PyErr_Occurred`, clears the indicator, and returns the exception as `*Error`. Use it after thin-layer calls that signal failure by a sentinel (nil pointer, -1 return, and so on). `IsPyException(err)` is a convenience wrapper around `errors.As`.

One other sentinel is worth mentioning: if you call an `Object` method on a nil receiver, the method returns `errNilObject` (unexported, check with `errors.Is` against the exported constant in a future release). That lets callers distinguish "Python raised" from "I forgot to import a module".

## Go / Python value conversions

`FromGo` and `ToGo` cover the types Go callers want to hand back and forth.

```go
func FromGo(v any) (*Object, error)
func ToGo[T any](o *Object) (T, error)
```

`FromGo` accepts:

* `nil`, which becomes Python `None` (with the right IncRef).
* `bool`.
* `int`, `int8`, `int16`, `int32`, `int64`, and the corresponding unsigned widths.
* `float32` and `float64`.
* `string`, which is decoded as UTF-8 into a `str`.
* `[]byte`, which becomes `bytes`.
* `[]any`, which becomes a `list` with each element recursively converted.
* `map[string]any`, which becomes a `dict` with string keys.

Unknown types return a Go error rather than panicking. The GIL must be held.

`ToGo[T]` is generic over the target type. Supported `T`:

* `bool` (uses `PyObject_IsTrue`, so it honors Python's truthiness).
* `int`, via `PyLong_AsLong`.
* `int64`, via `PyLong_AsLongLong`.
* `uint64`, via `PyLong_AsUnsignedLongLong`.
* `float64`, via `PyFloat_AsDouble`.
* `string`, via `str(o)` then UTF-8 decode. Works on any object, not just `str`.
* `[]byte`, which requires the input to be `bytes`.

Integer overflow and non-convertible objects surface as a `*Error` carrying the Python exception (`OverflowError`, `TypeError`, and so on). Unsupported `T` returns a plain Go error.

## Dropping to the thin layer

Every public C API function has a Go wrapper named after it. Reference counting is manual: `DecRef` releases, `IncRef` duplicates. The GIL must be held.

```go
defer python3.Acquire()()

mod := python3.PyImport_ImportModule("math")
if mod == nil {
    return python3.CheckError()
}
defer mod.DecRef()

pi := mod.GetAttrString("pi")
if pi == nil {
    return python3.CheckError()
}
defer pi.DecRef()

fmt.Println(python3.PyFloat_AsDouble(pi))
```

You can mix layers freely. `Object.Raw()` returns the underlying `*PyObject`; `newObject(p)` (unexported but reachable via `IncRef` patterns) wraps one back. The two are the same pointer.

The thin layer also exposes `PyConfig`, `PyStatus`, and the new `Py_InitializeFromConfig` surface. `Interp.New` uses them internally, so most callers do not need to touch them directly, but they are there if you need fine-grained control that the `Option` set does not cover.

## Subinterpreters and free-threaded builds

Subinterpreters (PEP 684 / PEP 734) and the free-threaded build (PEP 779, `python3.14t`) are in scope for this fork but not yet covered by the high-level layer. The thin-layer bindings for `PyInterpreterConfig`, `Py_NewInterpreterFromConfig`, `Py_EndInterpreter`, `PyUnstable_EnableTryIncRef`, and `PyMutex` are planned to land in follow-up commits tracked by `spec/0960_cpy3.md`.

## Testing

`go test ./...` runs against:

1. macOS arm64 with Homebrew `python@3.14` (GIL build).
2. Linux amd64 from a python.org source build (GIL build).
3. Linux amd64 with `--disable-gil`, invoked as `python3.14t`. The subinterpreter-per-GIL test is gated on this build.

The suite uses an internal `setupPy(t testing.TB)` helper that initializes the interpreter once per process, releases the GIL, and for each test pins the test goroutine and acquires the GIL with a `t.Cleanup` to release. A `TestMain` in `main_test.go` calls `runtime.LockOSThread` on the test runner's main goroutine as an additional safety net. Without these two pieces the suite would trip Python 3.12's strict GIL check under Go's goroutine migration and crash roughly one run in three.

A few tests are marked `t.Skip` in the shared-interpreter suite because they call `Py_Finalize` (or `Py_Main`, which does so internally) in the middle of the run. They still work in isolation, for example `go test -run '^TestInitialization$'`.

## Relation to upstream projects

[`go-python/cpy3`](https://github.com/go-python/cpy3) is the direct parent of this fork. It stopped receiving commits in 2022, still advertises Python 3.7 support, and does not build against 3.13 or newer because CPython removed `Py_SetProgramName`, `Py_SetPath`, `Py_SetPythonHome`, `Py_SetStandardStreamEncoding`, `PySys_SetArgv`, `PySys_SetArgvEx`, `PyEval_InitThreads`, and several internal helpers in 3.13. This fork rewrites those paths on top of `PyConfig`.

[`DataDog/go-python3`](https://github.com/DataDog/go-python3) was the previous upstream before it was archived in December 2021. `go-python/cpy3` positioned itself as a drop-in replacement; this fork keeps the same import path so the chain continues.

[`sbinet/go-python`](https://github.com/sbinet/go-python) is the Python 2 ancestor. The API design here is recognizable to anyone who used that package, though the 3.x C API is different enough that most function signatures are new.

## Contributing

Issues and pull requests welcome. The design notes and rollout plan for the 3.14 upgrade live at [`spec/0960_cpy3.md`](spec/0960_cpy3.md); read that before opening a PR that changes the C API surface. Style-wise, match the thin layer's existing convention: one Go function per public C function, named identically, with a doc comment that links to the CPython documentation.

## License

MIT. See [`LICENSE`](LICENSE). The fork keeps the Datadog copyright on files inherited from the original project; new files carry the current maintainer's copyright.
