// Copyright 2025 Upbound Inc.
// All rights reserved

package user

import (
	"context"
	"sort"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upterm"
)

// Member is a member from the API.
type Member struct {
	Member organizations.Member
	Invite organizations.Invite
}

const (
	statusActive  = "ACTIVE"
	statusInvited = "INVITED"
)

// listCmd lists users of an organization.
// It lists both members and invites.
type listCmd struct {
	OrgName string `arg:"" help:"Name of the organization." required:""`
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ResultPrinter, oc *organizations.Client) error {
	orgID, err := oc.GetOrgID(ctx, c.OrgName)
	if err != nil {
		return err
	}
	members, err := oc.ListMembers(ctx, orgID)
	if err != nil {
		return err
	}
	invites, err := oc.ListInvites(ctx, orgID)
	if err != nil {
		return err
	}

	// Create a full list of members & invites, sorted by username or email.
	allMembers := make([]Member, len(invites)+len(members))
	for i, invite := range invites {
		allMembers[i] = Member{Invite: invite}
	}
	for i, member := range members {
		allMembers[i+len(invites)] = Member{Member: member}
	}

	sort.SliceStable(allMembers, func(i, j int) bool {
		if allMembers[i].Member.User.Username != "" && allMembers[j].Member.User.Username != "" {
			return allMembers[i].Member.User.Username < allMembers[j].Member.User.Username
		}
		if allMembers[i].Member.User.Username != "" {
			return true
		}
		if allMembers[j].Member.User.Username != "" {
			return false
		}
		return allMembers[i].Invite.Email < allMembers[j].Invite.Email
	})

	listFieldNames := []string{"USERNAME", "NAME", "EMAIL", "PERMISSION", "STATUS"}
	return printer.PrintObject(allMembers, listFieldNames, extractMemberFields)
}

func extractMemberFields(obj any) []string {
	m, _ := obj.(Member)
	// If the user name exists, this is a member, not an invite.
	if m.Member.User.Username != "" {
		return []string{m.Member.User.Username, m.Member.User.Name, m.Member.User.Email, string(m.Member.Permission), statusActive}
	}
	// invites don't have usernames or names, so those are left blank.
	return []string{"", "", m.Invite.Email, string(m.Invite.Permission), statusInvited}
}
