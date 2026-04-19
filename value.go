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
)

// FromGo converts a Go value into a new Python Object. The caller owns
// the returned Object and must Close it.
//
// Supported input types: nil (returns Python None), bool, int, int8,
// int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32,
// float64, string, []byte, and slices/maps of any supported type.
//
// The GIL must be held.
func FromGo(v any) (*Object, error) {
	if v == nil {
		Py_None.IncRef()
		return newObject(Py_None), nil
	}
	switch x := v.(type) {
	case bool:
		n := 0
		if x {
			n = 1
		}
		return newObject(PyBool_FromLong(n)), nil
	case int:
		return newObject(PyLong_FromGoInt(x)), nil
	case int8:
		return newObject(PyLong_FromGoInt64(int64(x))), nil
	case int16:
		return newObject(PyLong_FromGoInt64(int64(x))), nil
	case int32:
		return newObject(PyLong_FromGoInt64(int64(x))), nil
	case int64:
		return newObject(PyLong_FromGoInt64(x)), nil
	case uint:
		return newObject(PyLong_FromGoUint(x)), nil
	case uint8:
		return newObject(PyLong_FromGoUint64(uint64(x))), nil
	case uint16:
		return newObject(PyLong_FromGoUint64(uint64(x))), nil
	case uint32:
		return newObject(PyLong_FromGoUint64(uint64(x))), nil
	case uint64:
		return newObject(PyLong_FromGoUint64(x)), nil
	case float32:
		return newObject(PyFloat_FromDouble(float64(x))), nil
	case float64:
		return newObject(PyFloat_FromDouble(x)), nil
	case string:
		return newObject(PyUnicode_FromString(x)), nil
	case []byte:
		return newObject(PyBytes_FromString(string(x))), nil
	case []any:
		return fromGoSlice(x)
	case map[string]any:
		return fromGoStringMap(x)
	}
	return nil, fmt.Errorf("python3: cannot convert Go %T to Python", v)
}

func fromGoSlice(vs []any) (*Object, error) {
	lst := PyList_New(len(vs))
	if lst == nil {
		return nil, errorFromPython()
	}
	for i, v := range vs {
		item, err := FromGo(v)
		if err != nil {
			lst.DecRef()
			return nil, err
		}
		// PyList_SetItem steals the reference.
		if PyList_SetItem(lst, i, item.Raw()) != 0 {
			item.Close()
			lst.DecRef()
			return nil, errorFromPython()
		}
	}
	return newObject(lst), nil
}

func fromGoStringMap(m map[string]any) (*Object, error) {
	d := PyDict_New()
	if d == nil {
		return nil, errorFromPython()
	}
	for k, v := range m {
		key := PyUnicode_FromString(k)
		if key == nil {
			d.DecRef()
			return nil, errorFromPython()
		}
		val, err := FromGo(v)
		if err != nil {
			key.DecRef()
			d.DecRef()
			return nil, err
		}
		if PyDict_SetItem(d, key, val.Raw()) != 0 {
			key.DecRef()
			val.Close()
			d.DecRef()
			return nil, errorFromPython()
		}
		key.DecRef()
		val.Close()
	}
	return newObject(d), nil
}

// ToGo converts a Python Object into the Go type T. Supported T are
// bool, int, int64, uint64, float64, string, []byte.
//
// The GIL must be held.
func ToGo[T any](o *Object) (T, error) {
	var zero T
	if o == nil {
		return zero, errNilObject
	}
	raw := o.Raw()

	var out any
	switch any(zero).(type) {
	case bool:
		out = raw.IsTrue() == 1
	case int:
		v := PyLong_AsLong(raw)
		if err := CheckError(); err != nil {
			return zero, err
		}
		out = v
	case int64:
		v := PyLong_AsLongLong(raw)
		if err := CheckError(); err != nil {
			return zero, err
		}
		out = v
	case uint64:
		v := PyLong_AsUnsignedLongLong(raw)
		if err := CheckError(); err != nil {
			return zero, err
		}
		out = v
	case float64:
		v := PyFloat_AsDouble(raw)
		if err := CheckError(); err != nil {
			return zero, err
		}
		out = v
	case string:
		s := raw.Str()
		if s == nil {
			return zero, errorFromPython()
		}
		defer s.DecRef()
		out = PyUnicode_AsUTF8(s)
	case []byte:
		if !PyBytes_Check(raw) {
			return zero, fmt.Errorf("python3: object is not bytes")
		}
		out = []byte(PyBytes_AsString(raw))
	default:
		return zero, fmt.Errorf("python3: unsupported target type %T", zero)
	}

	return out.(T), nil
}
