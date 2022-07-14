package client

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/tsundata/flowline/pkg/api/meta"
	"net/http"
	"testing"
)

func TestRestClient(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://127.0.0.1:5000/"
	uid := "a942c92c-edea-4052-bc9d-12db91e6b833"
	obj := meta.Workflow{}
	err := NewRestClient(baseURL).Request(ctx).
		Get().Workflow().UID(uid).Result().Into(&obj)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, obj.UID, uid)
}

func TestApiClient(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://127.0.0.1:5000/"
	res := NewApiClient(baseURL).Request(ctx).
		Post().Path("register").Data(meta.Worker{
		State:    meta.WorkerReady,
		Host:     "127.0.0.1",
		Port:     5001,
		Runtimes: []string{"javascript"},
	}).Result()
	if res.Error() != nil || res.statusCode != http.StatusOK {
		t.Fatal("error result")
	}

	require.Equal(t, res.Data(), []byte("OK"))
}
