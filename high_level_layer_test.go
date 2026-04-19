package python3

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunFile(t *testing.T) {
	setupPy(t)

	pyErr, err := PyRun_AnyFile("tests/test.py")
	assert.Zero(t, pyErr)
	assert.Nil(t, err)

	stdout := PySys_GetObject("stdout")

	result := stdout.CallMethodArgs("getvalue")
	defer result.DecRef()

	assert.Equal(t, "hello world\n", PyUnicode_AsUTF8(result))
}

func TestRunString(t *testing.T) {
	setupPy(t)

	pythonCode, err := ioutil.ReadFile("tests/test.py")
	assert.Nil(t, err)

	assert.Zero(t, PyRun_SimpleString(string(pythonCode)))

	stdout := PySys_GetObject("stdout")

	result := stdout.CallMethodArgs("getvalue")
	defer result.DecRef()

	assert.Equal(t, "hello world\n", PyUnicode_AsUTF8(result))
}

func TestPyMain(t *testing.T) {
	// Py_Main runs its own full interpreter lifecycle, including
	// Py_Initialize and Py_Finalize. Running it inside the shared
	// interpreter established by setupPy tears the interpreter down
	// under the other tests' feet. Skip it here; callers who want the
	// behavior should invoke Py_Main from a standalone program.
	t.Skip("Py_Main finalizes the interpreter; incompatible with the shared test interpreter")
}
