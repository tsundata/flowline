package handlers

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	// 34 chose as a number close to 30 that is likely to be unique enough to jump out at me the next time I see a timeout.
	// Everyone chooses 30.
	requestTimeoutUpperBound = 34 * time.Second
)

func limitedReadBody(req *http.Request, limit int64) ([]byte, error) {
	defer func() {
		_ = req.Body.Close()
	}()
	if limit <= 0 {
		return ioutil.ReadAll(req.Body)
	}
	lr := &io.LimitedReader{
		R: req.Body,
		N: limit + 1,
	}
	data, err := ioutil.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if lr.N <= 0 {
		return nil, fmt.Errorf("NewRequestEntityTooLargeError limit is %d", limit)
	}
	return data, nil
}

func hasUID(obj runtime.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}
	if len(accessor.GetUID()) == 0 {
		return false, nil
	}
	return true, nil
}
