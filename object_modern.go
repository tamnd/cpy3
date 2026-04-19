/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

/*
#include "Python.h"
*/
import "C"

// Object is an owning reference to a Python object. It is the modern
// type for callers who want an idiomatic Go handle; internally it is
// the same pointer as *PyObject, so the two can be converted freely.
//
// An Object must be released with Close. The GIL must be held on the
// calling thread (via python3.Acquire or an Interp method) for any
// Object method that talks to the interpreter.
type Object PyObject

// newObject wraps a freshly-owned *PyObject in an *Object. It returns
// nil when given a nil pointer so callers can forward Python "returns
// NULL on error" semantics.
func newObject(p *PyObject) *Object {
	if p == nil {
		return nil
	}
	return (*Object)(p)
}

// Raw returns the underlying *PyObject so callers can drop down to the
// thin C-API wrappers for operations the modern surface does not
// cover.
func (o *Object) Raw() *PyObject {
	if o == nil {
		return nil
	}
	return (*PyObject)(o)
}

// Close drops a reference (DecRef). Safe to call on a nil receiver.
// After Close the Object must not be used.
func (o *Object) Close() error {
	if o == nil {
		return nil
	}
	(*PyObject)(o).DecRef()
	return nil
}

// IncRef returns a new owning handle to the same underlying object.
// Use it when handing the value to a caller that will Close it while
// the original handle also wants to Close it.
func (o *Object) IncRef() *Object {
	if o == nil {
		return nil
	}
	(*PyObject)(o).IncRef()
	return o
}

// String returns the Python str() of the object, or an empty string
// on error. Implements fmt.Stringer.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	s := (*PyObject)(o).Str()
	if s == nil {
		return ""
	}
	defer s.DecRef()
	return PyUnicode_AsUTF8(s)
}

// Repr returns the Python repr() of the object, or an empty string on
// error.
func (o *Object) Repr() string {
	if o == nil {
		return "<nil>"
	}
	r := (*PyObject)(o).Repr()
	if r == nil {
		return ""
	}
	defer r.DecRef()
	return PyUnicode_AsUTF8(r)
}

// Type returns the fully qualified name of the object's Python type,
// for example "builtins.list" or "datetime.datetime".
func (o *Object) Type() string {
	if o == nil {
		return ""
	}
	t := (*PyObject)(o).Type()
	if t == nil {
		return ""
	}
	nameObj := (*PyObject)(t).GetAttrString("__name__")
	if nameObj == nil {
		return ""
	}
	defer nameObj.DecRef()
	return PyUnicode_AsUTF8(nameObj)
}

// GetAttr returns the named attribute on the object. The caller owns
// the result.
func (o *Object) GetAttr(name string) (*Object, error) {
	if o == nil {
		return nil, errNilObject
	}
	attr := (*PyObject)(o).GetAttrString(name)
	if attr == nil {
		return nil, errorFromPython()
	}
	return newObject(attr), nil
}

// SetAttr assigns value to the named attribute.
func (o *Object) SetAttr(name string, value *Object) error {
	if o == nil {
		return errNilObject
	}
	if (*PyObject)(o).SetAttrString(name, (*PyObject)(value)) != 0 {
		return errorFromPython()
	}
	return nil
}

// HasAttr reports whether the object has the named attribute.
func (o *Object) HasAttr(name string) bool {
	if o == nil {
		return false
	}
	return (*PyObject)(o).HasAttrString(name)
}

// Call calls the object as a function with the given positional
// arguments. The caller owns the result.
func (o *Object) Call(args ...*Object) (*Object, error) {
	if o == nil {
		return nil, errNilObject
	}
	tup := PyTuple_New(len(args))
	if tup == nil {
		return nil, errorFromPython()
	}
	defer tup.DecRef()
	for i, a := range args {
		// PyTuple_SetItem steals the reference, so IncRef the
		// caller's handle before handing it over.
		if a != nil {
			(*PyObject)(a).IncRef()
		}
		if PyTuple_SetItem(tup, i, (*PyObject)(a)) != 0 {
			return nil, errorFromPython()
		}
	}

	result := (*PyObject)(o).Call(tup, nil)
	if result == nil {
		return nil, errorFromPython()
	}
	return newObject(result), nil
}

// CallMethod calls the named method on the object.
func (o *Object) CallMethod(name string, args ...*Object) (*Object, error) {
	if o == nil {
		return nil, errNilObject
	}
	m, err := o.GetAttr(name)
	if err != nil {
		return nil, err
	}
	defer m.Close()
	return m.Call(args...)
}

// Len returns the object's length, like Python's len(). Returns -1 on
// error; use CheckError to fetch the underlying Python exception.
func (o *Object) Len() int {
	if o == nil {
		return -1
	}
	return (*PyObject)(o).Length()
}
