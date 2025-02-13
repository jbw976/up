// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
)

func GetOrganization(ctx context.Context, ac *accounts.Client, account string) (*accounts.AccountResponse, error) {
	a, err := ac.Get(ctx, account)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get Account %q", account)
	}
	if a.Account.Type != accounts.AccountOrganization {
		return nil, fmt.Errorf("account %q is not an organization", account)
	}
	if a.Organization == nil {
		return nil, fmt.Errorf("account %q does not have an organization", account)
	}
	return a, nil
}
