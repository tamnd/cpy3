# cpy3 — CPython 3.14 bindings for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/tamnd/cpy3.svg)](https://pkg.go.dev/github.com/tamnd/cpy3)

`cpy3` is a maintained fork of the [`go-python/cpy3`](https://github.com/go-python/cpy3) bindings, updated for CPython 3.14 and with an idiomatic Go API layer on top.

Two layers, same package:

- **Thin C-API wrappers** — one Go function per public `Py*_*` C function. Use these when you need something the high-level layer does not cover.
- **Idiomatic Go API** — an `Interp` handle, an owning `Object` type with `Close`, typed `Error` values with `errors.Unwrap` chaining, `FromGo` / `ToGo[T]` conversions, and a `python3.Acquire` helper that pins the goroutine and holds the GIL in one call.

## Install

Requires Go 1.23+ and CPython 3.14.0+ with `python-3.14-embed.pc` reachable from `pkg-config` (Homebrew `python@3.14` on macOS, `python3.14-dev` on Debian).

```bash
go get github.com/tamnd/cpy3
```

## Quick start

```go
package main

import (
    "fmt"

    python3 "github.com/tamnd/cpy3"
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

## GIL rules

Python 3.12 made every `Py_*` call from a thread that does not hold the GIL a hard abort. Go freely migrates goroutines across OS threads, so any Go code that touches the C API has to pin its goroutine and hold the GIL.

Use `Acquire`:

```go
defer python3.Acquire()()
// safe to call Object methods and thin C-API wrappers here
```

`Interp.Run`, `Interp.Import`, and `Interp.Eval` already do this internally. You only need `Acquire` when you read back Object methods or call `ToGo` after one of those returns.

## Error handling

Python exceptions come back as `*python3.Error` with `Type`, `Message`, and a `Cause` chain built from `__cause__` / `__context__`:

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

`python3.IsPyException(err)` is a convenience over `errors.As`.

## Calling into Python

```go
defer python3.Acquire()()

builtins, _ := p.Import("builtins")
defer builtins.Close()

length, _ := builtins.GetAttr("len")
defer length.Close()

arg, _ := python3.FromGo("hello")
defer arg.Close()

res, _ := length.Call(arg)
defer res.Close()

n, _ := python3.ToGo[int](res)
fmt.Println(n) // 5
```

## Thin C-API layer

Everything under the thin layer is named after its CPython counterpart and documented inline with a link to the CPython docs. You can drop down whenever the idiomatic surface does not cover a call:

```go
defer python3.Acquire()()
mod := python3.PyImport_ImportModule("math")
defer mod.DecRef()
pi := mod.GetAttrString("pi")
defer pi.DecRef()
fmt.Println(python3.PyFloat_AsDouble(pi))
```

## Status

Passes `go test ./...` against CPython 3.14 on macOS (arm64) and Linux (amd64 / arm64), GIL build. Free-threaded (`python3.14t`) build is in scope but gated behind a build tag.

See [`spec/0960_cpy3.md`](spec/0960_cpy3.md) for the full upgrade and API-layer design.

## License

MIT. See [`LICENSE`](LICENSE).
