package filters

import (
	"context"
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
	"regexp"
	"strings"
)

func WithJWT(handler http.Handler, secret []byte, whitelist []string) http.Handler {
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

		authHeader := req.Header.Get("Authorization")
		token, err := validJWT(authHeader, secret)
		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error jwt token"))
			return
		}
		if claims, ok := token.Claims.(*meta.UserClaims); ok {
			ctx := context.WithValue(req.Context(), constant.UserUID, claims.ID)
			req = req.WithContext(ctx)
			handler.ServeHTTP(w, req)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error jwt token"))
			return
		}
	})
}

func validJWT(authHeader string, secret []byte) (*jwt.Token, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errors.New("token error")
	}
	jwtToken := strings.Split(authHeader, " ")
	if len(jwtToken) < 2 {
		return nil, errors.New("token error")
	}

	return jwt.ParseWithClaims(jwtToken[1], &meta.UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
}

func whitelistRegexps(whitelist []string) []*regexp.Regexp {
	res, err := compileRegexps(whitelist)
	if err != nil {
		flog.Errorf("Invalid whitelist regex %v - %v", strings.Join(whitelist, ","), err)
	}
	return res
}
