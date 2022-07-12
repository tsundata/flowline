package uid

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUID(t *testing.T) {
	require.Len(t, New(), 36)
}
