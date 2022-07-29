package user

import (
	"errors"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

type UserStorage struct {
	REST *REST
}

func NewStorage(config *config.Config, options *options.StoreOptions) (UserStorage, error) {
	r, err := NewREST(config, options)
	if err != nil {
		return UserStorage{}, err
	}
	return UserStorage{REST: r}, nil
}

type REST struct {
	config *config.Config
	*registry.Store
}

func NewREST(config *config.Config, options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.User{} },
		NewListFunc:              func() runtime.Object { return &meta.UserList{} },
		NewStructFunc:            func() interface{} { return meta.User{} },
		NewListStructFunc:        func() interface{} { return meta.UserList{} },
		DefaultQualifiedResource: rest.Resource("user"),

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ResetFieldsStrategy: Strategy,
	}

	err := store.CompleteWithOptions(options)
	if err != nil {
		flog.Panic(err)
	}

	return &REST{config, store}, nil
}

func (r *REST) Actions() []rest.SubResourceAction {
	return []rest.SubResourceAction{
		{
			Verb:         "POST",
			SubResource:  "session",
			Params:       nil,
			ReadSample:   meta.UserSession{},
			WriteSample:  meta.UserSession{},
			ReturnSample: meta.UserSession{},
		},
		{
			Verb:         "DELETE",
			SubResource:  "session",
			Params:       nil,
			ReadSample:   meta.UserSession{},
			WriteSample:  meta.UserSession{},
			ReturnSample: meta.UserSession{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("POST", "session", sr.userLogin)
	srRoute.Match("DELETE", "session", sr.userLogout)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, errors.New("error subresource path"))
	}
}

type subResource struct {
	store *REST
}

func (r *subResource) userLogin(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	login := meta.UserSession{}
	err := req.ReadEntity(&login)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("form error"))
		return
	}

	obj, err := r.store.List(ctx, &meta.ListOptions{})
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("user error"))
		return
	}

	if list, ok := obj.(*meta.UserList); ok {
		var user meta.User
		for _, item := range list.Items {
			if item.Name == login.Username {
				user = item
				break
			}
		}
		if user.Name == "" {
			_ = resp.WriteError(http.StatusBadRequest, errors.New("username or password error"))
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password))
		if err != nil {
			_ = resp.WriteError(http.StatusBadRequest, errors.New("username or password error"))
			return
		}
		var jc = jwt.NewWithClaims(
			jwt.SigningMethodHS512,
			&meta.UserClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ID:        user.UID,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
					NotBefore: jwt.NewNumericDate(time.Now()),
				},
			},
		)
		if r.store.config.JWTSecret == "" {
			_ = resp.WriteError(http.StatusBadRequest, errors.New("token error"))
			return
		}

		secret := []byte(r.store.config.JWTSecret)
		token, err := jc.SignedString(secret)
		if err != nil {
			flog.Error(err)
			_ = resp.WriteError(http.StatusBadRequest, errors.New("token error"))
			return
		}

		_ = resp.WriteEntity(meta.UserSession{UserUID: user.UID, Token: token})
		return
	} else {
		_ = resp.WriteError(http.StatusBadRequest, errors.New("user error"))
		return
	}
}

func (r *subResource) userLogout(req *restful.Request, resp *restful.Response) {
	obj := meta.UserSession{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}
	fmt.Printf("%+v \n", obj)
	return
}
