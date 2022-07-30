package controller

// Interface defines the base of a controller managed by a controller manager
type Interface interface {
	// Name returns the canonical name of the controller.
	Name() string
}
