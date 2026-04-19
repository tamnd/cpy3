/*
Copyright 2026 Duc-Tam Nguyen. Licensed under the MIT License.
*/

package python3

import (
	"testing"
)

// TestDeferredAnnotations_NotEvaluatedEagerly exercises the PEP 649 /
// PEP 749 lazy annotation model that became the default in Python 3.14.
// Annotations referencing a name that is not defined when the class is
// constructed must not raise; they are evaluated on demand.
func TestDeferredAnnotations_NotEvaluatedEagerly(t *testing.T) {
	p := Default()
	if err := p.Run(`
class C:
    # NotYetDefined does not exist at class-creation time. Under
    # eager (pre-3.14) semantics this would raise NameError; under
    # PEP 649 it is captured as a string and resolved lazily.
    x: "NotYetDefined"
`); err != nil {
		t.Fatalf("class body should not raise under PEP 649: %v", err)
	}
}

// TestDeferredAnnotations_AnnotateAttr checks that the class carries a
// callable __annotate__ attribute, which is the PEP 649 hook used to
// (re)compute annotations on request.
func TestDeferredAnnotations_AnnotateAttr(t *testing.T) {
	p := Default()
	if err := p.Run(`
class C:
    x: int
    y: str
`); err != nil {
		t.Fatalf("Run: %v", err)
	}

	has, err := p.Eval("callable(C.__annotate__)")
	if err != nil {
		t.Fatalf("Eval callable(C.__annotate__): %v", err)
	}
	defer Acquire()()
	defer has.Close()

	ok, err := ToGo[bool](has)
	if err != nil {
		t.Fatalf("ToGo bool: %v", err)
	}
	if !ok {
		t.Fatalf("C.__annotate__ is not callable; PEP 649 expected")
	}
}

// TestDeferredAnnotations_LazyResolution defines an unquoted forward
// reference in an annotation and confirms that annotationlib can
// resolve it once the referenced symbol exists. Before PEP 649, this
// code raised NameError at class-definition time; the win of lazy
// annotations is that forward references no longer need the quoted
// "Target" workaround.
func TestDeferredAnnotations_LazyResolution(t *testing.T) {
	p := Default()
	if err := p.Run(`
import annotationlib

class Holder:
    ref: Target  # unquoted; would NameError under eager semantics

class Target:
    pass

anns = annotationlib.get_annotations(Holder, format=annotationlib.Format.VALUE)
resolved = anns["ref"]
`); err != nil {
		t.Fatalf("Run: %v", err)
	}

	v, err := p.Eval("resolved is Target")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	defer Acquire()()
	defer v.Close()

	ok, err := ToGo[bool](v)
	if err != nil {
		t.Fatalf("ToGo bool: %v", err)
	}
	if !ok {
		t.Fatalf("forward reference did not resolve to Target")
	}
}
