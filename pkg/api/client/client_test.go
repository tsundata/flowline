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
	obj := meta.Workflow{}
	err := NewRestClient(baseURL).Request(ctx).
		Get().Workflow().List().Result().Into(&obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestApiClient(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://127.0.0.1:5000/"
	res := NewApiClient(baseURL).Request(ctx).
		Post().Path("register").Data(meta.Worker{
		State:    meta.WorkerReady,
		Host:     "127.0.0.1:5001",
		Runtimes: []string{"javascript"},
	}).Result()
	if res.Error() != nil || res.statusCode != http.StatusOK {
		t.Fatal("error result")
	}

	require.Equal(t, res.Data(), []byte("OK"))
}
