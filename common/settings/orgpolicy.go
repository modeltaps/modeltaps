package settings

// Settings page group "orgpolicy" (root only).
func init() {
	add(
		Setting{
			Key: "OrganizationEnabled", Scope: ScopeSite, Group: "orgpolicy", Card: "org-policy", Type: TypeBool,
			Label: "Organizations",
			Description: "Users can create and join organizations that share a balance, members, budgets and API keys. " +
				"When off, users no longer see organizations and the organization API refuses every request; site admins " +
				"still manage the existing ones, which are kept as they are.",
			Related:  []string{"OrganizationMaxPerUser", "OrganizationDefaultMaxMembers", "OrganizationQuotaTransferEnabled"},
			Keywords: []string{"teams", "workspaces", "multi-tenant", "org"},
		},
		Setting{
			Key: "OrganizationQuotaTransferEnabled", Scope: ScopeSite, Group: "orgpolicy", Card: "org-policy", Type: TypeBool,
			Label: "Personal balance transfer into organizations",
			Description: "An organization's owner or admins can move their own personal balance into the organization's " +
				"shared balance. The transfer is one-way: nothing moves back to a personal account. When off, transfers are " +
				"refused.",
			Related:  []string{"OrganizationEnabled", "OrganizationDissolveQuotaRefund"},
			Keywords: []string{"transfer", "quota", "balance", "top up organization", "pool"},
		},
		Setting{
			Key: "OrganizationMaxPerUser", Scope: ScopeSite, Group: "orgpolicy", Card: "org-policy", Type: TypeInt,
			Min:   num(0),
			Label: "Organizations per user",
			Description: "How many organizations one user can create. Organizations they only joined do not count, nor " +
				"deleted ones. 0 means no limit.",
			Examples: []Example{
				{Value: "3", Effect: "A user who created 3 organizations must delete one before creating a fourth; joining others is not limited."},
				{Value: "1", Effect: "Each user runs at most one organization of their own."},
				{Value: "0", Effect: "Users create as many organizations as they like."},
			},
			Related:  []string{"OrganizationEnabled", "OrganizationDefaultMaxMembers"},
			Keywords: []string{"limit", "maximum organizations", "quota"},
		},
		Setting{
			Key: "OrganizationDefaultMaxMembers", Scope: ScopeSite, Group: "orgpolicy", Card: "org-policy", Type: TypeInt,
			Min:   num(0),
			Label: "Default member limit",
			Description: "How many members an organization can have when a site admin has not set a limit for that " +
				"organization. Changing it applies to every organization without a limit of its own, existing ones included. " +
				"0 means no limit.",
			Examples: []Example{
				{Value: "10", Effect: "An organization with 10 members cannot add an 11th until a site admin raises its limit."},
				{Value: "0", Effect: "Organizations without a limit of their own take any number of members."},
			},
			Related:  []string{"OrganizationMaxPerUser", "OrganizationEnabled"},
			Keywords: []string{"seats", "members", "team size", "limit"},
		},
		Setting{
			Key: "OrganizationDissolveQuotaRefund", Scope: ScopeSite, Group: "orgpolicy", Card: "org-policy", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "owner", Label: "Return to the owner", Description: "The remaining shared balance goes to the owner's personal account, with a log entry."},
				{Value: "discard", Label: "Discard", Description: "The remaining shared balance is cleared and returned to nobody."},
			},
			Label: "Balance of a deleted organization",
			Description: "What happens to an organization's remaining shared balance when it is deleted, by its owner or a " +
				"site admin. It has no field on the settings page and is only set through the option API (PUT /api/option/). " +
				"Any value other than owner discards the balance.",
			Related:  []string{"OrganizationQuotaTransferEnabled", "OrganizationEnabled"},
			Keywords: []string{"dissolve", "delete organization", "refund", "leftover balance"},
			Advanced: true,
		},
	)
}
