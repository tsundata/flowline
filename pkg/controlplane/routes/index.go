package routes

import (
	"github.com/gorilla/mux"
	"net/http"
)

type Index struct{}

func (i Index) Install(mux *mux.Router) {
	mux.HandleFunc("/", i.Index)
}

func (i Index) Index(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("apiserver"))
}
