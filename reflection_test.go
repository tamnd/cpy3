package python3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReflectionBuiltins(t *testing.T) {
	setupPy(t)

	builtins := PyEval_GetBuiltins()
	assert.NotNil(t, builtins)

	len := PyDict_GetItemString(builtins, "len")
	assert.True(t, PyCallable_Check(len))
}

func TestReflectionLocals(t *testing.T) {
	setupPy(t)

	locals := PyEval_GetLocals()
	assert.Nil(t, locals)
}

func TestReflectionGlobals(t *testing.T) {
	setupPy(t)

	globals := PyEval_GetGlobals()
	assert.Nil(t, globals)
}

func TestReflectionFuncName(t *testing.T) {
	setupPy(t)

	builtins := PyEval_GetBuiltins()
	assert.NotNil(t, builtins)

	len := PyDict_GetItemString(builtins, "len")
	assert.True(t, PyCallable_Check(len))

	assert.Equal(t, "len", PyEval_GetFuncName(len))
}
func TestReflectionFuncDesc(t *testing.T) {
	setupPy(t)

	builtins := PyEval_GetBuiltins()
	assert.NotNil(t, builtins)

	len := PyDict_GetItemString(builtins, "len")
	assert.True(t, PyCallable_Check(len))

	assert.Equal(t, "()", PyEval_GetFuncDesc(len))
}
