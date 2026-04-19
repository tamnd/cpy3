/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

import (
	"strings"
	"testing"
)

// The modern API tests live as a group so they share the Default
// interpreter. Individual tests Acquire the GIL as they need it.

func TestInterp_RunAndEval(t *testing.T) {
	p := Default()
	if err := p.Run("x = 6 * 7"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	v, err := p.Eval("x")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	defer Acquire()()
	defer v.Close()

	got, err := ToGo[int](v)
	if err != nil {
		t.Fatalf("ToGo: %v", err)
	}
	if got != 42 {
		t.Fatalf("x = %d, want 42", got)
	}
}

func TestInterp_Import(t *testing.T) {
	p := Default()
	m, err := p.Import("sys")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	defer m.Close()

	defer Acquire()()

	ver, err := m.GetAttr("version")
	if err != nil {
		t.Fatalf("GetAttr: %v", err)
	}
	defer ver.Close()

	if s := ver.String(); s == "" {
		t.Fatalf("sys.version was empty")
	}
}

func TestInterp_RunSyntaxError(t *testing.T) {
	p := Default()
	err := p.Run("this is not python(")
	if err == nil {
		t.Fatalf("Run should have returned an error")
	}
}

func TestError_TypeAndMessage(t *testing.T) {
	p := Default()
	_, err := p.Eval("1/0")
	if err == nil {
		t.Fatalf("Eval 1/0 should have failed")
	}
	if !IsPyException(err) {
		t.Fatalf("err is not a Python exception: %v", err)
	}
	if !strings.Contains(err.Error(), "ZeroDivisionError") {
		t.Fatalf("expected ZeroDivisionError in %q", err.Error())
	}
}

func TestObject_Stringer(t *testing.T) {
	defer Acquire()()

	o := newObject(PyUnicode_FromString("héllo"))
	defer o.Close()

	if got := o.String(); got != "héllo" {
		t.Fatalf("String() = %q, want %q", got, "héllo")
	}
	if got := o.Type(); got != "str" {
		t.Fatalf("Type() = %q, want %q", got, "str")
	}
}

func TestFromGo_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   any
	}{
		{"int", 12345},
		{"float", 3.14},
		{"string", "héllo"},
		{"bool true", true},
		{"bool false", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer Acquire()()
			o, err := FromGo(tc.in)
			if err != nil {
				t.Fatalf("FromGo: %v", err)
			}
			defer o.Close()
			if o.String() == "" && tc.in != "" {
				t.Fatalf("round-trip of %v produced empty string", tc.in)
			}
		})
	}
}

func TestObject_Call(t *testing.T) {
	p := Default()
	builtins, err := p.Import("builtins")
	if err != nil {
		t.Fatalf("Import builtins: %v", err)
	}
	defer builtins.Close()

	defer Acquire()()

	length, err := builtins.GetAttr("len")
	if err != nil {
		t.Fatalf("GetAttr len: %v", err)
	}
	defer length.Close()

	arg, err := FromGo("hello")
	if err != nil {
		t.Fatalf("FromGo: %v", err)
	}
	defer arg.Close()

	res, err := length.Call(arg)
	if err != nil {
		t.Fatalf("Call len: %v", err)
	}
	defer res.Close()

	n, err := ToGo[int](res)
	if err != nil {
		t.Fatalf("ToGo: %v", err)
	}
	if n != 5 {
		t.Fatalf("len('hello') = %d, want 5", n)
	}
}

func TestAcquire_Reentrant(t *testing.T) {
	release1 := Acquire()
	defer release1()

	release2 := Acquire()
	release2()

	// Reaching this line without the process aborting is the test:
	// nested Acquire must not break the GIL accounting.
}
