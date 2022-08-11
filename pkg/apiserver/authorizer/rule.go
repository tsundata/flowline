package authorizer

type ResourceRuleInfo interface {
	// GetVerbs returns a list of kubernetes resource API verbs.
	GetVerbs() []string
	// GetAPIGroups return the names of the APIGroup that contains the resources.
	GetAPIGroups() []string
	// GetResources return a list of resources the rule applies to.
	GetResources() []string
	// GetResourceNames return a white list of names that the rule applies to.
	GetResourceNames() []string
}

// DefaultResourceRuleInfo holds information that describes a rule for the resource
type DefaultResourceRuleInfo struct {
	Verbs         []string
	APIGroups     []string
	Resources     []string
	ResourceNames []string
}

func (i *DefaultResourceRuleInfo) GetVerbs() []string {
	return i.Verbs
}

func (i *DefaultResourceRuleInfo) GetAPIGroups() []string {
	return i.APIGroups
}

func (i *DefaultResourceRuleInfo) GetResources() []string {
	return i.Resources
}

func (i *DefaultResourceRuleInfo) GetResourceNames() []string {
	return i.ResourceNames
}

type NonResourceRuleInfo interface {
	// GetVerbs returns a list of kubernetes resource API verbs.
	GetVerbs() []string
	// GetNonResourceURLs return a set of partial urls that a user should have access to.
	GetNonResourceURLs() []string
}

// DefaultNonResourceRuleInfo holds information that describes a rule for the non-resource
type DefaultNonResourceRuleInfo struct {
	Verbs           []string
	NonResourceURLs []string
}

func (i *DefaultNonResourceRuleInfo) GetVerbs() []string {
	return i.Verbs
}

func (i *DefaultNonResourceRuleInfo) GetNonResourceURLs() []string {
	return i.NonResourceURLs
}
