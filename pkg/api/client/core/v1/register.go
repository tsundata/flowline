package v1

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: constant.GroupName, Version: "v1"}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(schema *runtime.Scheme) error {
	schema.AddKnownTypes(SchemeGroupVersion,
		&meta.Code{},
		&meta.CodeList{},
		&meta.Connection{},
		&meta.ConnectionList{},
		&meta.Dag{},
		&meta.DagList{},
		&meta.Event{},
		&meta.EventList{},
		&meta.Job{},
		&meta.JobList{},
		&meta.Role{},
		&meta.RoleList{},
		&meta.Stage{},
		&meta.StageList{},
		&meta.User{},
		&meta.UserList{},
		&meta.Variable{},
		&meta.VariableList{},
		&meta.Worker{},
		&meta.WorkerList{},
		&meta.Workflow{},
		&meta.WorkflowList{},
	)
	schema.AddKnownTypes(SchemeGroupVersion, &meta.Status{})

	return nil
}

var Scheme = runtime.NewScheme()

func init() {
	_ = AddToScheme(Scheme)
}
