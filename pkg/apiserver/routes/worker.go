package routes

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/worker"
	"github.com/tsundata/flowline/pkg/util/flog"
	"io/ioutil"
	"net/http"
)

type Worker struct {
	Storage rest.Storage
}

func (w Worker) Install(mux *mux.Router) {
	mux.HandleFunc("/heartbeat", w.Heartbeat)
	mux.HandleFunc("/register", w.Register)
}

func (w Worker) Heartbeat(rw http.ResponseWriter, req *http.Request) {
	// todo lease
	_, _ = rw.Write([]byte("OK"))
}

func (w Worker) Register(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	if r, ok := w.Storage.(*worker.REST); ok {
		obj := r.New()
		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			flog.Error(err)
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(data, &obj)
		if err != nil {
			flog.Error(err)
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		_, err = r.Create(context.Background(), obj, nil, nil)
		if err != nil {
			flog.Error(err)
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	_, _ = rw.Write([]byte("OK"))
}
