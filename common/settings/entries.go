package settings

// Settings page group "entries".
func init() {
	add(
		Setting{
			Key: "BuiltinChatEnabled", Scope: ScopeSite, Group: "entries", Card: "builtin-chat", Type: TypeBool,
			Label: "Built-in chat",
			Description: "Lets users try models in the console's playground and shows the quick-start chat on the dashboard. " +
				"Off keeps the playground pages visible but disables running a request, and points users to the API docs " +
				"instead. API keys and requests from other clients are not affected.",
			Keywords: []string{"playground", "chat", "try models", "web chat"},
		},
	)
}
