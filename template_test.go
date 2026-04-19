/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

import (
	"strings"
	"testing"
)

// TestTemplateString_Basic evaluates a PEP 750 template string inside
// the default interpreter and checks that the resulting object carries
// the expected structure: a tuple of literal strings and a tuple of
// Interpolation objects carrying the captured values.
//
// PEP 750 does not expose a stable C API for template objects; the
// public surface lives in the Python standard library (string.templatelib).
// This test therefore drives the feature through the interpreter rather
// than through a cgo wrapper, which is also how end users will consume
// it from Go code.
func TestTemplateString_Basic(t *testing.T) {
	p := Default()
	if err := p.Run(`
from string.templatelib import Template, Interpolation
name = "world"
tpl = t"hello {name}!"
assert isinstance(tpl, Template)
pieces = tuple(tpl)
`); err != nil {
		t.Fatalf("run template setup: %v", err)
	}

	tpl, err := p.Eval("tpl")
	if err != nil {
		t.Fatalf("eval tpl: %v", err)
	}
	defer Acquire()()
	defer tpl.Close()

	if got := tpl.Type(); got != "Template" {
		t.Fatalf("type(tpl) = %q, want Template", got)
	}

	strs, err := tpl.GetAttr("strings")
	if err != nil {
		t.Fatalf("GetAttr strings: %v", err)
	}
	defer strs.Close()
	if !strings.Contains(strs.Repr(), "hello ") || !strings.Contains(strs.Repr(), "!") {
		t.Fatalf("unexpected strings tuple: %s", strs.Repr())
	}

	interps, err := tpl.GetAttr("interpolations")
	if err != nil {
		t.Fatalf("GetAttr interpolations: %v", err)
	}
	defer interps.Close()
	if n := interps.Len(); n != 1 {
		t.Fatalf("len(interpolations) = %d, want 1", n)
	}
}

// TestTemplateString_FormatSpec verifies that a format spec attached to
// an interpolation ({value:.2f}) survives into the Interpolation
// object. PEP 750 exposes the spec verbatim so downstream tooling can
// decide how to render it.
func TestTemplateString_FormatSpec(t *testing.T) {
	p := Default()
	if err := p.Run(`
value = 3.14159
tpl = t"pi = {value:.2f}"
interp = tpl.interpolations[0]
`); err != nil {
		t.Fatalf("run: %v", err)
	}

	spec, err := p.Eval("interp.format_spec")
	if err != nil {
		t.Fatalf("eval format_spec: %v", err)
	}
	defer Acquire()()
	defer spec.Close()

	if got, _ := ToGo[string](spec); got != ".2f" {
		t.Fatalf("format_spec = %q, want %q", got, ".2f")
	}
}
