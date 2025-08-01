// Copyright 2025 Upbound Inc.
// All rights reserved

package organization

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthentication "k8s.io/client-go/pkg/apis/clientauthentication/v1"

	"github.com/upbound/up-sdk-go/service/auth"
	"github.com/upbound/up/internal/upbound"
)

// tokenCmd generates an org-scoped token for use with spaces.
type tokenCmd struct {
	Name  string `arg:""         env:"ORGANIZATION"                                                                help:"Name of organization." predictor:"orgs" required:""`
	Token string `env:"UP_TOKEN" help:"Token used to execute command. Overrides the token present in the profile." short:"t"`
}

// Run executes the token command.
func (c *tokenCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKAuthConfig()
	if err != nil {
		return err
	}

	sessionToken := c.Token
	if sessionToken == "" {
		sessionToken = upCtx.Profile.Session
	}

	client := auth.NewClient(cfg)
	orgToken, err := client.GetOrgScopedToken(ctx, c.Name, sessionToken)
	if err != nil {
		return err
	}

	exp := v1.NewTime(time.Now().Add(time.Duration(orgToken.ExpiresIn) * time.Second))

	creds := clientauthentication.ExecCredential{
		TypeMeta: v1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: clientauthentication.SchemeGroupVersion.String(),
		},
		Status: &clientauthentication.ExecCredentialStatus{
			ExpirationTimestamp: &exp,
			Token:               orgToken.AccessToken,
		},
	}

	out, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	p.Print(string(out))
	return nil
}
