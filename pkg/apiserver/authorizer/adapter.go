package authorizer

import (
	"context"
	"errors"
	casbinModel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"strings"
)

const (
	PLACEHOLDER = "_"
)

type Adapter struct {
	storage *registry.DryRunnableStorage
}

func NewAdapter(storage *registry.DryRunnableStorage) *Adapter {
	return &Adapter{storage: storage}
}

func (a *Adapter) LoadPolicy(model casbinModel.Model) error {
	ctx := context.Background()
	obj := &meta.PolicyList{}
	err := a.storage.GetList(ctx, rest.WithPrefix("policy"), meta.ListOptions{}, obj)
	if err != nil {
		return err
	}

	for _, policy := range obj.Items {
		a.loadPolicy(policy, model)
	}

	return nil
}

func (a *Adapter) loadPolicy(policy meta.Policy, model casbinModel.Model) {
	lineText := strings.Builder{}
	lineText.WriteString(policy.PType)
	if policy.V0 != "" {
		lineText.WriteString(", ")
		lineText.WriteString(policy.V0)
	}
	if policy.V1 != "" {
		lineText.WriteString(", ")
		lineText.WriteString(policy.V1)
	}
	if policy.V2 != "" {
		lineText.WriteString(", ")
		lineText.WriteString(policy.V2)
	}
	if policy.V3 != "" {
		lineText.WriteString(", ")
		lineText.WriteString(policy.V3)
	}
	if policy.V4 != "" {
		lineText.WriteString(", ")
		lineText.WriteString(policy.V4)
	}
	if policy.V5 != "" {
		lineText.WriteString(", ")
		lineText.WriteString(policy.V5)
	}
	persist.LoadPolicyLine(lineText.String(), model)
}

func (a *Adapter) SavePolicy(model casbinModel.Model) error {
	obj := &meta.PolicyList{}
	var policy []meta.Policy

	for ptype, ast := range model["p"] {
		for _, line := range ast.Policy {
			policy = append(policy, a.convertRule(ptype, line))
		}
	}
	for ptype, ast := range model["g"] {
		for _, line := range ast.Policy {
			policy = append(policy, a.convertRule(ptype, line))
		}
	}
	obj.Items = policy

	return a.savePolicy(obj)
}

func (a *Adapter) savePolicy(obj *meta.PolicyList) error {
	ctx := context.Background()
	for i, policy := range obj.Items {
		key := rest.WithPrefix("policy/" + policy.Key)
		err := a.storage.Get(ctx, key, meta.GetOptions{}, &obj.Items[i])
		if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
			return err
		}
		if errors.Is(err, storage.ErrKeyNotFound) {
			err = a.storage.Create(ctx, key, &policy, &obj.Items[i], 0, false)
		} else {
			err = a.storage.GuaranteedUpdate(ctx, key, &policy, false, nil, nil, false, nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) convertRule(ptype string, line []string) meta.Policy {
	policy := meta.Policy{}
	policy.PType = ptype
	policys := []string{ptype}
	length := len(line)

	if len(line) > 0 {
		policy.V0 = line[0]
		policys = append(policys, line[0])
	}
	if len(line) > 1 {
		policy.V1 = line[1]
		policys = append(policys, line[1])
	}
	if len(line) > 2 {
		policy.V2 = line[2]
		policys = append(policys, line[2])
	}
	if len(line) > 3 {
		policy.V3 = line[3]
		policys = append(policys, line[3])
	}
	if len(line) > 4 {
		policy.V4 = line[4]
		policys = append(policys, line[4])
	}
	if len(line) > 5 {
		policy.V5 = line[5]
		policys = append(policys, line[5])
	}

	for i := 0; i < 6-length; i++ {
		policys = append(policys, PLACEHOLDER)
	}

	policy.Key = strings.Join(policys, "/")

	return policy
}

func (a *Adapter) AddPolicy(sec string, ptype string, rule []string) error {
	panic("implement me")
}

func (a *Adapter) RemovePolicy(sec string, ptype string, rule []string) error {
	panic("implement me")
}

func (a *Adapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	panic("implement me")
}
