package user

import (
	"context"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/authorizer"
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/xerrors"
	"net/http"
	"sort"
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

	// BeginCreate
	store.BeginCreate = func(ctx context.Context, obj runtime.Object, options *meta.CreateOptions) (registry.FinishFunc, error) {
		user, ok := obj.(*meta.User)
		if !ok {
			return nil, xerrors.New("error user info")
		}

		listObj, err := store.List(ctx, &meta.ListOptions{})
		if err != nil {
			return nil, err
		}

		if userList, ok := listObj.(*meta.UserList); ok {
			for _, item := range userList.Items {
				if user.Name == item.Name {
					return nil, xerrors.New("Duplicate username")
				}
			}
		}

		// password
		password, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
		if err != nil {
			return nil, err
		}
		user.Password = string(password)

		return func(ctx context.Context, success bool) {}, nil
	}

	// BeginUpdate
	store.BeginUpdate = func(ctx context.Context, obj, old runtime.Object, options *meta.UpdateOptions) (registry.FinishFunc, error) {
		user, ok := obj.(*meta.User)
		if !ok {
			return nil, xerrors.New("error user info")
		}
		oldUser, ok := old.(*meta.User)
		if !ok {
			return nil, xerrors.New("error user info")
		}

		// username
		if user.Name != oldUser.Name {
			listObj, err := store.List(ctx, &meta.ListOptions{})
			if err != nil {
				return nil, err
			}

			if userList, ok := listObj.(*meta.UserList); ok {
				for _, item := range userList.Items {
					if user.Name == item.Name {
						return nil, xerrors.New("Duplicate username")
					}
				}
			}
			return func(ctx context.Context, success bool) {}, nil
		}

		// password
		if user.Password != oldUser.Password {
			password, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
			if err != nil {
				return nil, err
			}
			user.Password = string(password)
		}

		return func(ctx context.Context, success bool) {}, nil
	}

	// AfterUpdate
	store.AfterUpdate = func(obj runtime.Object, options *meta.UpdateOptions) {
		enforcer, err := authorizer.NewEnforcer(authorizer.NewAdapter(&store.Storage))
		enforcer.EnableAutoSave(false)
		if err != nil {
			flog.Error(err)
			return
		}
		if user, ok := obj.(*meta.User); ok {
			_, err = enforcer.RemoveFilteredGroupingPolicy(0, user.UID)
			if err != nil {
				flog.Error(err)
				return
			}
			for _, role := range user.Roles {
				_, err = enforcer.AddGroupingPolicy(user.UID, role)
				if err != nil {
					flog.Error(err)
					return
				}
			}
			err = enforcer.SavePolicy()
			if err != nil {
				flog.Error(err)
				return
			}
		}
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
		{
			Verb:         "GET",
			SubResource:  "dashboard",
			Params:       nil,
			ReturnSample: meta.Dashboard{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("POST", "session", sr.userLogin)
	srRoute.Match("DELETE", "session", sr.userLogout)
	srRoute.Match("GET", "dashboard", sr.dashboard)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("error subresource path"))
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
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("form error"))
		return
	}

	obj, err := r.store.List(ctx, &meta.ListOptions{})
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("user error"))
		return
	}

	list, ok := obj.(*meta.UserList)
	if !ok {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("user error"))
		return
	}

	var user meta.User
	for _, item := range list.Items {
		if item.Name == login.Username {
			user = item
			break
		}
	}
	if user.Name == "" {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("username or password error"))
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password))
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("username or password error"))
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
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("token error"))
		return
	}

	secret := []byte(r.store.config.JWTSecret)
	token, err := jc.SignedString(secret)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("token error"))
		return
	}

	_ = resp.WriteEntity(meta.UserSession{UserUID: user.UID, Token: token})
	return
}

func (r *subResource) userLogout(req *restful.Request, _ *restful.Response) {
	obj := meta.UserSession{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}
	fmt.Printf("%+v \n", obj)
}

func (r *subResource) dashboard(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()

	workflowAmount, err := r.store.Storage.Count(rest.WithPrefix("workflow"))
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("count workflow error"))
		return
	}
	codeAmount, err := r.store.Storage.Count(rest.WithPrefix("code"))
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("count code error"))
		return
	}
	variableAmount, err := r.store.Storage.Count(rest.WithPrefix("variable"))
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("count variable error"))
		return
	}
	workerAmount, err := r.store.Storage.Count(rest.WithPrefix("worker"))
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("count worker error"))
		return
	}

	// Scheduled
	list := meta.EventList{}
	err = r.store.Storage.GetList(ctx, rest.WithPrefix("event"), meta.ListOptions{}, &list)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("get events error"))
		return
	}

	dateCountMap := make(map[string]int)
	for _, item := range list.Items {
		if item.Reason == "Scheduled" {
			date := item.CreationTimestamp.Format("2006-01-02")
			dateCountMap[date] += 1
		}
	}
	resultData := DashboardDataArray{}
	for date, count := range dateCountMap {
		resultData = append(resultData, meta.DashboardData{
			Date:     date,
			Schedule: count,
		})
	}
	sort.Sort(resultData)

	_ = resp.WriteEntity(meta.Dashboard{
		WorkflowAmount: workflowAmount,
		CodeAmount:     codeAmount,
		VariableAmount: variableAmount,
		WorkerAmount:   workerAmount,
		Data:           resultData,
	})
}

type DashboardDataArray []meta.DashboardData

func (d DashboardDataArray) Len() int {
	return len(d)
}

func (d DashboardDataArray) Less(i, j int) bool {
	return d[i].Date < d[j].Date
}

func (d DashboardDataArray) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
