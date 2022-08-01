package uid

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUID(t *testing.T) {
	require.Len(t, New(), 36)
}

func TestIsValid(t *testing.T) {
	require.True(t, IsValid(New()))
}
