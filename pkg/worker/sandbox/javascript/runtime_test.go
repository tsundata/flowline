package javascript

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRuntime(t *testing.T) {
	js := NewRuntime()
	out, err := js.Run(`
parseInt(input() + 1)
`, 1000)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, out, int64(1001))
}
