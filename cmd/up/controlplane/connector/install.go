// Copyright 2025 Upbound Inc.
// All rights reserved

package connector

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/version"
)

var mcpRepoURL = urlMustParse("xpkg.upbound.io/spaces-artifacts") //nolint:gochecknoglobals // Would make this a const if we could.

const (
	connectorName = "mcp-connector"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply() error {
	if c.ClusterName == "" {
		c.ClusterName = c.Namespace
	}
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		helm.IsOCI(),
		helm.WithNamespace(c.InstallationNamespace),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.mgr = mgr
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client

	base := map[string]any{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck // Can't do anything useful with this error.
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	c.parser = helm.NewParser(base, c.Set)
	return nil
}

// installCmd connects the current cluster to a control plane in an account on
// Upbound.
type installCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Name      string `arg:"" help:"Name of control plane."                                                         predictor:"ctps" required:""`
	Namespace string `arg:"" help:"Namespace in the control plane where the claims of the cluster will be stored." required:""`

	Token                 string `help:"API token used to authenticate. If not provided, a new robot and a token will be created."`
	ClusterName           string `help:"Name of the cluster connecting to the control plane. If not provided, the namespace argument value will be used."`
	Kubeconfig            string `help:"Override the default kubeconfig path."                                                                            type:"existingfile"`
	InstallationNamespace string `default:"kube-system"                                                                                                   env:"MCP_CONNECTOR_NAMESPACE" help:"Kubernetes namespace for MCP Connector. Default is kube-system." short:"n"`
	ControlPlaneSecret    string `help:"Name of the secret that contains the kubeconfig for a control plane."`

	install.CommonParams
}

// Run executes the connect command.
func (c *installCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	token, err := c.getToken(p, upCtx)
	if err != nil {
		return errors.Wrap(err, "failed to get token")
	}
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	// Some of these settings are only applicable if pointing to an Upbound
	// Cloud control plane. We leave them consistent since they won't impact
	// our ability to point the connector at Space control plane.
	params["mcp"] = map[string]any{
		"account":   upCtx.Organization,
		"name":      c.Name,
		"namespace": c.Namespace,
		"host":      fmt.Sprintf("%s://%s", upCtx.ProxyEndpoint.Scheme, upCtx.ProxyEndpoint.Host),
		"token":     token,
	}

	// If the control-plane-secret has been specified, disable provisioning
	// the mcp-kubeconfig secret in favor of the supplied secret name.
	if c.ControlPlaneSecret != "" {
		v := params["mcp"]
		param, ok := v.(map[string]any)
		if !ok {
			return errors.New("expected mcp params to be a map")
		}
		param["secret"] = map[string]any{
			"name":      c.ControlPlaneSecret,
			"provision": false,
		}

		params["mcp"] = param
	}

	p.Printfln("Installing %s to kube-system. This may take a few minutes.", connectorName)
	if err = c.mgr.Install(version.MCPConnectorVersion(), params); err != nil {
		return err
	}

	if _, err = c.mgr.GetCurrentVersion(); err != nil {
		return err
	}

	p.Printfln("Connected to the control plane %s.", c.Name)
	p.Println("See available APIs with the following command: \n\n$ kubectl api-resources")
	return nil
}

func (c *installCmd) getToken(p pterm.TextPrinter, upCtx *upbound.Context) (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to build SDK config")
	}
	// NOTE(muvaf): We always use the querying user's account to create a token
	// assuming it has enough privileges. The ideal is to create a robot and a
	// token when the "--account" flag points to an organization but the robots
	// don't have default permissions, hence it'd require creation of team,
	// membership and also control plane permission to make it work.
	//
	// This is why this command is currently under alpha because we need to be
	// able to connect for organizations in a scalable way, i.e. every cluster
	// should have its own robot account.
	a, err := accounts.NewClient(cfg).Get(context.Background(), upCtx.Profile.ID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get account details")
	}
	p.Printfln("Creating an API token for the user %s. This token will be "+
		"used to authenticate the cluster.", a.Account.Name)
	resp, err := tokens.NewClient(cfg).Create(context.Background(), &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.ClusterName,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerUser,
					ID:   strconv.FormatUint(uint64(a.Organization.CreatorID), 10),
				},
			},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to create token")
	}
	p.Printfln("Created a token named %s.", c.ClusterName)
	return fmt.Sprint(resp.DataSet.Meta["jwt"]), nil
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
