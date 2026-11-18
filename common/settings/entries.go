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
		Setting{
			Key: "ChatLinks", Scope: ScopeSite, Group: "entries", Card: "chat-links", Type: TypeJSON,
			Label: "Chat links",
			Description: "The chat clients listed for users, each with a name, an address, a sort order and whether it shows. " +
				"Stored as a JSON array of {id, name, url, show, sort}; the card edits one entry at a time and saves it right " +
				"away. The list is published in the site status for clients that read it; no API key is added to the links. " +
				"Empty means no chat links.",
			Examples: []Example{
				{Value: `[{"id":"a1","name":"Web chat","url":"https://chat.example.com","show":true,"sort":1}]`, Effect: "One visible entry named Web chat."},
				{Value: `[{"id":"a1","name":"Web chat","url":"https://chat.example.com","show":false,"sort":1}]`, Effect: "The entry is kept but hidden."},
			},
			Related:  []string{"ChatLink", "BuiltinChatEnabled"},
			Keywords: []string{"chat client", "chat entry", "web chat", "external chat"},
		},
	)
}
