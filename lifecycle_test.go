/*
Unless explicitly stated otherwise all files in this repository are licensed
under the MIT License.
This product includes software developed at Datadog (https://www.datadoghq.com/).
Copyright 2018 Datadog, Inc.
*/

package python3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests that cycle Py_Initialize / Py_Finalize are destructive to the
// shared test interpreter. They remain here for documentation but are
// skipped in the main suite; verify them manually with `go test -run`
// in isolation.

func TestInitialization(t *testing.T) {
	t.Skip("destructive: finalizes the shared test interpreter")
	Py_Initialize()
	assert.True(t, Py_IsInitialized())
	Py_Finalize()
	assert.False(t, Py_IsInitialized())
}

func TestInitializationEx(t *testing.T) {
	t.Skip("destructive: finalizes the shared test interpreter")
	Py_Initialize()
	assert.True(t, Py_IsInitialized())
	assert.Zero(t, Py_FinalizeEx())
	assert.False(t, Py_IsInitialized())
}

func TestPyConfigProgramName(t *testing.T) {
	t.Skip("destructive: finalizes the shared test interpreter")
	Py_Finalize()

	cfg := NewPyConfig(PyConfigPython)
	defer cfg.Clear()

	name := "py3é"
	assert.True(t, cfg.SetProgramName(name).IsOk())
	assert.Equal(t, name, cfg.ProgramName())
}

func TestPyConfigPythonHome(t *testing.T) {
	t.Skip("destructive: finalizes the shared test interpreter")
	Py_Finalize()

	cfg := NewPyConfig(PyConfigPython)
	defer cfg.Clear()

	home := "høme"
	assert.True(t, cfg.SetPythonHome(home).IsOk())
	assert.Equal(t, home, cfg.PythonHome())
}

func TestPrefix(t *testing.T) {
	setupPy(t)
	prefix, err := Py_GetPrefix()
	assert.Nil(t, err)
	assert.IsType(t, "", prefix)
}

func TestExecPrefix(t *testing.T) {
	setupPy(t)
	execPrefix, err := Py_GetExecPrefix()
	assert.Nil(t, err)
	assert.IsType(t, "", execPrefix)
}

func TestProgramFullPath(t *testing.T) {
	setupPy(t)
	programFullPath, err := Py_GetProgramFullPath()
	assert.Nil(t, err)
	assert.IsType(t, "", programFullPath)
}

func TestVersion(t *testing.T) {
	version := Py_GetVersion()
	assert.IsType(t, "", version)
}

func TestPlatform(t *testing.T) {
	platform := Py_GetPlatform()
	assert.IsType(t, "", platform)
}

func TestCopyright(t *testing.T) {
	copyright := Py_GetCopyright()
	assert.IsType(t, "", copyright)
}

func TestCompiler(t *testing.T) {
	compiler := Py_GetCompiler()
	assert.IsType(t, "", compiler)
}

func TestBuildInfo(t *testing.T) {
	buildInfo := Py_GetBuildInfo()
	assert.IsType(t, "", buildInfo)
}

func TestPyConfigSetArgv(t *testing.T) {
	t.Skip("destructive: finalizes and re-initializes the interpreter")
	Py_Finalize()

	cfg := NewPyConfig(PyConfigIsolated)
	assert.True(t, cfg.SetArgv([]string{"test.py"}, false).IsOk())
	assert.True(t, Py_InitializeFromConfig(cfg).IsOk())
	cfg.Clear()

	argv := PySys_GetObject("argv")
	assert.Equal(t, 1, PyList_Size(argv))
	assert.Equal(t, "test.py", PyUnicode_AsUTF8(PyList_GetItem(argv, 0)))

	Py_Finalize()
}

func TestPyConfigInitFromConfig(t *testing.T) {
	t.Skip("destructive: finalizes and re-initializes the interpreter")
	Py_Finalize()

	cfg := NewPyConfig(PyConfigPython)
	assert.True(t, cfg.SetProgramName("cpy3-test").IsOk())
	assert.True(t, Py_InitializeFromConfig(cfg).IsOk())
	cfg.Clear()

	assert.True(t, Py_IsInitialized())
	Py_Finalize()
}
