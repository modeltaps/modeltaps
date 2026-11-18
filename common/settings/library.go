package settings

// The model library's inclusion rules. They are site options, but they are edited on the Models
// page (Inclusion rules tab), not on the settings page; the entries point there.
func init() {
	add(
		Setting{
			Key: "ModelLibraryRulesEnabled", Scope: ScopeSite, Group: "gateway", Card: "supply-intake-rules", Type: TypeBool,
			Label: "Run the inclusion rules",
			Description: "Runs the inclusion rules after each daily model library sync, so new releases join the catalog on their own. " +
				"Edited on the Models page, Inclusion rules tab. Off, the library still syncs but nothing " +
				"is added or retired automatically.",
			Related:  []string{"ModelLibraryRulesApply", "ModelLibraryRules"},
			Keywords: []string{"model library", "inclusion rules", "auto add", "new models", "catalog sync"},
			Advanced: true,
		},
		Setting{
			Key: "ModelLibraryRulesApply", Scope: ScopeSite, Group: "gateway", Card: "supply-intake-rules", Type: TypeBool,
			Label: "Apply the inclusion rules",
			Description: "Lets the inclusion rules change the catalog: on, they add and retire models and notify admins of what they " +
				"did. Off is a dry run: they change nothing and only notify admins of what they would do. Edited on the Models " +
				"page, Inclusion rules tab.",
			Examples: []Example{
				{Value: "false", Effect: "A new Claude release is reported as \"would be enabled\"; the catalog stays as it is."},
				{Value: "true", Effect: "The same release joins the catalog enabled, and admins are told it was added."},
			},
			Related:  []string{"ModelLibraryRulesEnabled", "ModelLibraryRules"},
			Keywords: []string{"dry run", "model library", "inclusion rules", "auto publish"},
			Advanced: true,
		},
		Setting{
			Key: "ModelLibraryRules", Scope: ScopeSite, Group: "gateway", Card: "supply-intake-rules", Type: TypeJSON,
			Label: "Inclusion rules",
			Description: "The rules that decide which library models join the catalog and which catalog models are retired. " +
				"They are saved as a JSON list and edited on the Models page, Inclusion rules tab. Rules are tried in order and the first one whose " +
				"conditions all hold decides: enable, add disabled, leave out or retire. Conditions are labs, id patterns, " +
				"release age, whether a channel can supply the model, deprecated, open weights and endpoints. Each model is " +
				"decided once, so a model you remove is not added back. Empty uses the built-in rules; an empty list does nothing.",
			Examples: []Example{
				{Value: `[{"name":"Retire deprecated models","deprecated":true,"action":"retire"}]`, Effect: "Deprecated catalog models stop being listed but stay callable; nothing is added."},
				{Value: `[{"name":"New Anthropic models","labs":["anthropic"],"released_within_days":60,"supplied":true,"action":"publish"}]`, Effect: "An Anthropic model released in the last 60 days that a channel offers joins the catalog enabled."},
				{Value: "", Effect: "The built-in rules: retire deprecated models, add previews disabled, enable recent models of the main labs, add other supplied models disabled."},
			},
			Related:  []string{"ModelLibraryRulesEnabled", "ModelLibraryRulesApply", "SupplyIntakeRules"},
			Keywords: []string{"model library", "inclusion rules", "auto add", "retire", "models.dev"},
			Advanced: true,
		},
	)
}
