package python3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeCheck(t *testing.T) {
	setupPy(t)

	assert.True(t, PyType_Check(Type))
	assert.True(t, PyType_CheckExact(Type))
}
