package routes

import (
	"github.com/gorilla/mux"
	"github.com/tsundata/flowline/pkg/util/version"
	"net/http"
)

type Index struct{}

func (i Index) Install(mux *mux.Router) {
	mux.HandleFunc("/", i.Index)
}

func (i Index) Index(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("Flowline " + version.Version))
}
