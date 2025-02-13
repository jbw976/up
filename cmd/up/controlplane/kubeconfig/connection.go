// Copyright 2025 Upbound Inc.
// All rights reserved

package kubeconfig

// ConnectionSecretCmd is the base for command getting connection secret for a control plane.
type ConnectionSecretCmd struct {
	Name  string `arg:""                                                                                                                                 help:"Name of control plane." predictor:"ctps" required:""`
	Token string `help:"API token used to authenticate. Required for Upbound Cloud; ignored otherwise."`
	Group string `help:"The control plane group that the control plane is contained in. By default, this is the group specified in the current profile." short:"g"`
}
