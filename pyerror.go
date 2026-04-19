/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

/*
#include "Python.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Error represents a raised Python exception as a Go error. The fields
// are snapshots of PyErr_GetRaisedException at the moment the error
// was captured; the Python state has been cleared by that point, so
// callers may raise a new exception without interference.
type Error struct {
	// Type is the class of the raised exception, for example
	// "builtins.TypeError" or "my_mod.MyError".
	Type string

	// Message is str(exception) at the time of capture.
	Message string

	// Cause is the chained cause, populated from __cause__ or
	// __context__, if any.
	Cause *Error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil python error>"
	}
	if e.Type == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Type
	}
	return e.Type + ": " + e.Message
}

// Unwrap returns the chained Python cause, if any, so errors.Is and
// errors.As can walk the chain.
func (e *Error) Unwrap() error {
	if e == nil || e.Cause == nil {
		return nil
	}
	return e.Cause
}

// errNilObject is returned when a method is called on a nil Object.
// It is a distinct error (not an *Error) so callers can tell apart
// "the Python call raised" from "I forgot to import a module".
var errNilObject = errors.New("python3: nil Object")

// CheckError fetches the current Python exception, if any, clears it,
// and returns it as a Go error. Returns nil when no exception is set.
// The GIL must be held.
func CheckError() error {
	if C.PyErr_Occurred() == nil {
		return nil
	}
	return errorFromPython()
}

// errorFromPython pulls the current raised exception out of the
// interpreter, clears the error indicator, and returns it as *Error.
// The GIL must be held.
//
// The function walks __cause__ (PEP 3134 explicit chaining) then
// __context__ (implicit chaining) to populate Cause. CPython 3.12
// introduced PyErr_GetRaisedException as the single-object accessor;
// we use it here rather than the legacy three-slot API.
func errorFromPython() *Error {
	exc := C.PyErr_GetRaisedException()
	if exc == nil {
		return &Error{Message: "python3: error indicator not set"}
	}
	return unpackError(exc)
}

func unpackError(exc *C.PyObject) *Error {
	if exc == nil {
		return nil
	}
	defer C.Py_DecRef(exc)

	e := &Error{Type: pyExcTypeName(exc), Message: pyStr(exc)}

	// Walk __cause__ first, then __context__.
	for _, attr := range [...]string{"__cause__", "__context__"} {
		c := pyGetAttrBorrowed(exc, attr)
		if c == nil || unsafe.Pointer(c) == unsafe.Pointer(C.Py_None) {
			if c != nil {
				C.Py_DecRef(c)
			}
			continue
		}
		// PyObject_GetAttrString returned a new reference; take
		// ownership so we can recurse and release.
		e.Cause = unpackError(c)
		break
	}

	return e
}

func pyExcTypeName(exc *C.PyObject) string {
	typ := C.PyObject_Type(exc)
	if typ == nil {
		return ""
	}
	defer C.Py_DecRef(typ)

	var name string
	if n := pyGetAttrBorrowed(typ, "__module__"); n != nil {
		if s := pyStr(n); s != "" && s != "builtins" {
			name = s + "."
		}
		C.Py_DecRef(n)
	}
	if n := pyGetAttrBorrowed(typ, "__qualname__"); n != nil {
		name += pyStr(n)
		C.Py_DecRef(n)
	}
	return name
}

func pyStr(o *C.PyObject) string {
	s := C.PyObject_Str(o)
	if s == nil {
		C.PyErr_Clear()
		return ""
	}
	defer C.Py_DecRef(s)

	var size C.Py_ssize_t
	cstr := C.PyUnicode_AsUTF8AndSize(s, &size)
	if cstr == nil {
		C.PyErr_Clear()
		return ""
	}
	return C.GoStringN(cstr, C.int(size))
}

func pyGetAttrBorrowed(o *C.PyObject, name string) *C.PyObject {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	a := C.PyObject_GetAttrString(o, cname)
	if a == nil {
		C.PyErr_Clear()
	}
	return a
}

// IsPyException reports whether err wraps a raised Python exception.
// It is a convenience around errors.As.
func IsPyException(err error) bool {
	var e *Error
	return errors.As(err, &e)
}

