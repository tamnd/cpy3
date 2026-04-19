package python3

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSysGetSetObject(t *testing.T) {
	setupPy(t)

	platform := PySys_GetObject("platform")
	assert.NotNil(t, platform)
	assert.True(t, PyUnicode_Check(platform))
	platform.IncRef()

	newPlatform := PyUnicode_FromString("test")
	defer newPlatform.DecRef()

	assert.Zero(t, PySys_SetObject("platform", newPlatform))

	assert.Equal(t, newPlatform, PySys_GetObject("platform"))

	assert.Zero(t, PySys_SetObject("platform", platform))
}

// Since 3.13, PySys_AddWarnOption, PySys_ResetWarnOptions, PySys_AddXOption
// and PySys_SetPath are gone. Use PyConfig instead.

func TestSysPathViaPyConfig(t *testing.T) {
	t.Skip("destructive: finalizes and re-initializes the interpreter")
	// Bring up the default interpreter first so we can read back the
	// stdlib path it picked, then tear it down and reinitialize with a
	// PyConfig whose module_search_paths starts with our own entry.
	// Using PyConfigIsolated + a single made-up path breaks Python's
	// ability to import the encodings module.
	setupPy(t)
	stdPaths := make([]string, 0, 8)
	path := PySys_GetObject("path")
	for i := 0; i < PyList_Size(path); i++ {
		stdPaths = append(stdPaths, PyUnicode_AsUTF8(PyList_GetItem(path, i)))
	}
	Py_Finalize()

	cfg := NewPyConfig(PyConfigIsolated)
	paths := append([]string{"test"}, stdPaths...)
	if s := cfg.SetModuleSearchPaths(paths); !s.IsOk() {
		t.Fatalf("SetModuleSearchPaths: %v", s.Err())
	}
	if s := Py_InitializeFromConfig(cfg); !s.IsOk() {
		cfg.Clear()
		t.Fatalf("Py_InitializeFromConfig: %v", s.Err())
	}
	cfg.Clear()

	// Python resolves relative entries in module_search_paths to
	// absolute paths, so just verify our entry ended up at index 0.
	path = PySys_GetObject("path")
	first := PyUnicode_AsUTF8(PyList_GetItem(path, 0))
	assert.True(t, strings.HasSuffix(first, "test"), first)

	Py_Finalize()
}
