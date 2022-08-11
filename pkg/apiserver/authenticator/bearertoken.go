package authenticator

import (
	"github.com/tsundata/flowline/pkg/apiserver/warning"
	"golang.org/x/xerrors"
	"net/http"
	"strings"
)

const (
	invalidTokenWithSpaceWarning = "the provided Authorization header contains extra space before the bearer token, and is ignored"
)

var invalidToken = xerrors.New("invalid bearer token")

type BearerToken struct {
	auth Token
}

func NewBearerToken(auth Token) *BearerToken {
	return &BearerToken{auth: auth}
}

func (a *BearerToken) AuthenticateRequest(req *http.Request) (*Response, bool, error) {
	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if auth == "" {
		return nil, false, nil
	}
	parts := strings.SplitN(auth, " ", 3)
	if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, false, nil
	}

	token := parts[1]

	if len(token) == 0 {
		if len(parts) == 3 {
			warning.AddWarning(req.Context(), "", invalidTokenWithSpaceWarning)
		}
		return nil, false, nil
	}

	resp, ok, err := a.auth.AuthenticateToken(req.Context(), token)
	if ok {
		req.Header.Del("Authorization")
	}
	if !ok && err == nil {
		err = invalidToken
	}

	return resp, ok, err
}
