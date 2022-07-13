package event

// deploymentStrategy implements behavior for Deployments.
type strategy struct{}

// Strategy is the default logic that applies when creating and updating
// objects via the REST API.
var Strategy = strategy{}

func (d strategy) GetResetFields() map[string]interface{} {
	//TODO implement me
	panic("implement me")
}
