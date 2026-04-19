package python3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGIL(t *testing.T) {
	setupPy(t)

	gil := PyGILState_Ensure()

	assert.True(t, PyGILState_Check())

	PyGILState_Release(gil)
}

func TestThreadState(t *testing.T) {
	setupPy(t)

	threadState := PyGILState_GetThisThreadState()

	threadState2 := PyThreadState_Get()

	assert.Equal(t, threadState, threadState2)

	threadState3 := PyThreadState_Swap(threadState)

	assert.Equal(t, threadState, threadState3)
}

func TestThreadSaveRestore(t *testing.T) {
	setupPy(t)

	threadState := PyEval_SaveThread()

	assert.False(t, PyGILState_Check())

	PyEval_RestoreThread(threadState)
}
