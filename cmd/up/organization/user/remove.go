// Copyright 2025 Upbound Inc.
// All rights reserved

package user

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/input"
)

// removeCmd removes a user from an organization.
// The user can be specified by username or email address.
// If the user has been invited (but not yet joined) the invite is removed.
// If the user is a member of the organization, the user is removed.
type removeCmd struct {
	prompter input.Prompter

	OrgName string `arg:"" help:"Name of the organization."                required:""`
	User    string `arg:"" help:"Username or email of the user to remove." required:""`

	Force bool `default:"false" help:"Force removal of the member."`
}

const (
	errUserNotFound = "user not found"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *removeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *removeCmd) AfterApply() error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to remove this member? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the remove command.
func (c *removeCmd) Run(ctx context.Context, p pterm.TextPrinter, oc *organizations.Client) error {
	orgID, err := oc.GetOrgID(ctx, c.OrgName)
	if err != nil {
		return err
	}

	// First try to remove an invite.
	inviteID, err := findInviteID(ctx, oc, orgID, c.User)
	if err == nil {
		if err = oc.DeleteInvite(ctx, orgID, inviteID); err != nil {
			return err
		}

		p.Printfln("Invite for %s removed from %s", c.User, c.OrgName)
		return nil
	}

	// If no invite was found, try to remove a member.
	userID, err := findUserID(ctx, oc, orgID, c.User)
	if err == nil {
		if err = oc.RemoveMember(ctx, orgID, userID); err != nil {
			return err
		}
		p.Printfln("Member %s removed from %s", c.User, c.OrgName)
		return nil
	}

	return errors.New(errUserNotFound)
}

// findInviteID returns the invite ID for the given email address, if it exists.
func findInviteID(ctx context.Context, oc *organizations.Client, orgID uint, email string) (uint, error) {
	invites, err := oc.ListInvites(ctx, orgID)
	if err != nil {
		return 0, err
	}
	for _, invite := range invites {
		if invite.Email == email {
			return invite.ID, nil
		}
	}
	return 0, errors.New(errUserNotFound)
}

// findUserID returns the user ID for the given username or email address, if it exists.
func findUserID(ctx context.Context, oc *organizations.Client, orgID uint, username string) (uint, error) {
	users, err := oc.ListMembers(ctx, orgID)
	if err != nil {
		return 0, err
	}
	for _, user := range users {
		if user.User.Username == username || user.User.Email == username {
			return user.User.ID, nil
		}
	}
	return 0, errors.New(errUserNotFound)
}
