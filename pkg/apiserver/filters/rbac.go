package filters

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/tsundata/flowline/pkg/apiserver/authenticator/user"
	"github.com/tsundata/flowline/pkg/apiserver/authorizer"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
	"strings"
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

		resource, subresource := parseResource(req)
		attributes := authorizer.AttributesRecord{
			User: &user.DefaultInfo{
				UID: uid,
			},
			Verb:            parseVerb(req),
			APIGroup:        constant.GroupName,
			APIVersion:      constant.Version,
			Resource:        resource,
			Subresource:     subresource,
			ResourceRequest: true,
			Path:            req.URL.Path,
		}
		decision, reason, err := enforcer.Authorize(req.Context(), attributes)
		flog.Debugf("RBAC: %s %s %s ~ %d", uid, resource, parseVerb(req), decision)
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

func parseVerb(req *http.Request) string {
	switch req.Method {
	case http.MethodGet:
		if strings.HasSuffix(req.URL.Path, "list") {
			return "LIST"
		}
		if strings.HasSuffix(req.URL.Path, "watch") {
			return "WATCH"
		}
		return "GET"
	case http.MethodPost:
		return "POST"
	case http.MethodPut:
		return "PUT"
	case http.MethodDelete:
		// or DELETECOLLECTION
		return "DELETE"
	case http.MethodPatch:
		return "PATCH"
	}
	return ""
}

func parseResource(req *http.Request) (string, string) {
	paths := strings.Split(req.URL.Path, "/")
	var resources []string
	for _, path := range paths {
		if path == constant.RestPrefix || path == constant.GroupName || path == constant.Version {
			continue
		}
		if _, err := uuid.Parse(path); err == nil {
			continue
		}
		if path == "" {
			continue
		}
		resources = append(resources, path)
	}
	if len(resources) >= 2 {
		return resources[0], resources[1]
	}
	if len(resources) >= 1 {
		return resources[0], ""
	}
	return "", ""
}
