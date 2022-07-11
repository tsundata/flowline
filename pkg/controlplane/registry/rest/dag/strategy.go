package dag

// deploymentStrategy implements behavior for Deployments.
type dagStrategy struct{}

// Strategy is the default logic that applies when creating and updating
// objects via the REST API.
var Strategy = dagStrategy{}

func (d dagStrategy) GetResetFields() map[string]interface{} {
	//TODO implement me
	panic("implement me")
}
