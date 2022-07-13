package client

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/tsundata/flowline/pkg/api/meta"
	"testing"
)

func TestRESTClient(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://127.0.0.1:5000/apis/"
	uid := "a942c92c-edea-4052-bc9d-12db91e6b833"
	obj := meta.Workflow{}
	err := New(baseURL).Request(ctx).
		Get().Workflow().UID(uid).Result().Into(&obj)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, obj.UID, uid)
}
