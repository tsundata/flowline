package filters

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
	"strings"
)

func WithJWT(handler http.Handler, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if !validJWT(authHeader, secret) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("error jwt token"))
			return
		}
		handler.ServeHTTP(w, req)
	})
}

func validJWT(authHeader string, secret string) bool {
	if !strings.HasPrefix(authHeader, "JWT ") {
		return false
	}
	jwtToken := strings.Split(authHeader, " ")
	if len(jwtToken) < 2 {
		return false
	}
	parts := strings.Split(jwtToken[1], ".")
	err := jwt.SigningMethodHS512.Verify(strings.Join(parts[0:2], "."), parts[2], secret)
	if err != nil {
		flog.Warnf("error jwt token %s", err)
		return false
	}
	return false
}
