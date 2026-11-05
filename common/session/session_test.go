package session

import "testing"

func TestExtractSessionIDFromMetadataValue(t *testing.T) {
	const sid = "26668c3b-0fc7-46f6-9a9e-a47e3b53676d"
	cases := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"legacy string", "user_abc_account__session_" + sid, sid},
		{"object", map[string]interface{}{"device_id": "d", "session_id": sid}, sid},
		{"json-encoded object string", `{"device_id":"d","account_uuid":"","session_id":"` + sid + `"}`, sid},
		{"json string without session", `{"device_id":"d"}`, ""},
		{"plain user id", "user-42", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := ExtractSessionIDFromMetadataValue(tc.value); got != tc.want {
			t.Fatalf("%s: want %q, got %q", tc.name, tc.want, got)
		}
	}
}
