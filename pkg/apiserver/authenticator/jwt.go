package authenticator

import (
	"context"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/authenticator/user"
	"golang.org/x/xerrors"
)

type JWT struct {
	Secret []byte
}

func NewJWT(secret []byte) *JWT {
	return &JWT{Secret: secret}
}

func (j *JWT) AuthenticateToken(ctx context.Context, token string) (*Response, bool, error) {
	t, err := jwt.ParseWithClaims(token, &meta.UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.Secret, nil
	})
	if err != nil {
		return nil, false, err
	}

	if !t.Valid {
		return nil, false, xerrors.New("jwt not valid")
	}

	public, ok := t.Claims.(*meta.UserClaims)
	if !ok {
		return nil, false, xerrors.New("error jwt token")
	}

	tokenAudiences := Audiences(public.Audience)

	requestAudiences, ok := AudiencesFrom(ctx)
	if !ok {
		requestAudiences = []string{}
	}

	auds := Audiences(tokenAudiences).Intersect(requestAudiences)

	return &Response{
		Audiences: auds,
		User: &user.DefaultInfo{
			UID: public.ID,
		},
	}, true, nil
}
