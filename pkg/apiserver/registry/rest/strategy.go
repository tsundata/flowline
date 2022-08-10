package rest

import (
	"context"
	"github.com/tsundata/flowline/pkg/runtime"
)

type DefaultCreateStrategy struct{}

func (d DefaultCreateStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {}

func (d DefaultCreateStrategy) Validate(ctx context.Context, obj runtime.Object) []error {
	return nil
}

func (d DefaultCreateStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

type DefaultResetFieldsStrategy struct{}

func (d DefaultResetFieldsStrategy) GetResetFields() map[string]interface{} {
	return nil
}

type DefaultRESTUpdateStrategy struct {
}

func (d DefaultRESTUpdateStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (d DefaultRESTUpdateStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {}

func (d DefaultRESTUpdateStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) []error {
	return nil
}

func (d DefaultRESTUpdateStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (d DefaultRESTUpdateStrategy) AllowUnconditionalUpdate() bool {
	return false
}

type DefaultRESTCanonicalize struct {
}

func (d DefaultRESTCanonicalize) Canonicalize(obj runtime.Object) {
}
