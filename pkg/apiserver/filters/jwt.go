package filters

import (
	"context"
	"github.com/tsundata/flowline/pkg/apiserver/authenticator"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
	"regexp"
	"strings"
)

func WithJWT(handler http.Handler, secret string, whitelist []string) http.Handler {
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

		// authenticator
		auth := authenticator.NewBearerToken(authenticator.NewJWT([]byte(secret)))
		resp, ok, err := auth.AuthenticateRequest(req)
		if err != nil || !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error jwt token"))
			return
		}
		if resp != nil && resp.User.GetUID() != "" {
			ctx := context.WithValue(req.Context(), constant.UserUID, resp.User.GetUID())
			req = req.WithContext(ctx)
			handler.ServeHTTP(w, req)
		}
	})
}

func whitelistRegexps(whitelist []string) []*regexp.Regexp {
	res, err := compileRegexps(whitelist)
	if err != nil {
		flog.Errorf("Invalid whitelist regex %v - %v", strings.Join(whitelist, ","), err)
	}
	return res
}
