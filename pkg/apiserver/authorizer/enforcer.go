package authorizer

import (
	"context"
	"github.com/casbin/casbin/v2"
	casbinModel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/tsundata/flowline/pkg/apiserver/authenticator/user"
)

type Enforcer struct {
	*casbin.CachedEnforcer
}

func NewEnforcer(a persist.Adapter) (*Enforcer, error) {
	m, err := casbinModel.NewModelFromString(model)
	if err != nil {
		return nil, err
	}
	e, err := casbin.NewCachedEnforcer(m, a)
	if err != nil {
		return nil, err
	}
	return &Enforcer{e}, nil
}

func (e *Enforcer) Authorize(_ context.Context, a Attributes) (authorized Decision, reason string, err error) {
	ok, err := e.Enforce(a.GetUser().GetUID(), a.GetResource(), a.GetSubresource(), a.GetVerb())
	if err != nil {
		return 0, "", err
	}
	if ok {
		return DecisionAllow, "", nil
	}
	return DecisionDeny, "", nil
}

func (e *Enforcer) RulesFor(user user.Info) ([]ResourceRuleInfo, []NonResourceRuleInfo, bool, error) {
	return nil, nil, false, nil
}
