package code

import (
	v1 "github.com/tsundata/flowline/pkg/api/client/core/v1"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
)

// strategy implements behavior for rest.
type strategy struct {
	runtime.ObjectTyper

	rest.DefaultCreateStrategy
	rest.DefaultResetFieldsStrategy
	rest.DefaultRESTUpdateStrategy
	rest.DefaultRESTCanonicalize
}

// Strategy is the default logic that applies when creating and updating
// objects via the REST API.
var Strategy = strategy{ObjectTyper: v1.Scheme}
