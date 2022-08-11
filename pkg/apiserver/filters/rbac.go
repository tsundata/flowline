package filters

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/apiserver/authenticator/user"
	"github.com/tsundata/flowline/pkg/apiserver/authorizer"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
)

func WithRBAC(handler http.Handler, storage *registry.DryRunnableStorage, whitelist []string) http.Handler {
	whitelistREs := whitelistRegexps(whitelist)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		allowed := false
		for _, re := range whitelistREs {
			if allowed = re.MatchString(req.URL.Path); allowed {
				break
			}
		}
		if allowed || req.Method == "OPTIONS" {
			handler.ServeHTTP(w, req)
			return
		}

		enforcer, err := authorizer.NewEnforcer(authorizer.NewAdapter(storage))
		if err != nil {
			flog.Error(err)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error rbac"))
			return
		}

		uid, ok := req.Context().Value(constant.UserUID).(string)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error user"))
			return
		}

		decision, reason, err := enforcer.Authorize(req.Context(), authorizer.AttributesRecord{
			User: &user.DefaultInfo{
				UID: uid,
			},
			Verb:            "", // todo
			APIGroup:        constant.GroupName,
			APIVersion:      constant.Version,
			Resource:        "", // todo
			Subresource:     "", // todo
			Name:            "", // todo
			ResourceRequest: true,
			Path:            req.URL.Path,
		})
		if err != nil {
			flog.Error(err)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error user"))
			return
		}
		switch decision {
		case authorizer.DecisionAllow:
			handler.ServeHTTP(w, req)
		case authorizer.DecisionDeny, authorizer.DecisionNoOpinion:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(fmt.Sprintf("authorizer error %s", reason)))
			return
		}
	})
}
