package router

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// The answer that hands an agent its key names the account the key belongs to, so the agent can
// show it: whoever answers a leaked user code first chooses the account, and the agent's traffic
// then lands in that account's logs.

func agentKeyAccount(t *testing.T, w mkResponse) map[string]any {
	t.Helper()
	account, _ := w.body["account"].(map[string]any)
	if w.status != http.StatusOK || account == nil {
		t.Fatalf("the key answer should name its account: %d %s", w.status, w.raw)
	}
	return account
}

func TestDeviceFlowNamesTheApprovingAccount(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	ip := "198.51.100.81"
	start := f.agentCall(http.MethodPost, "/api/agent/device", ip, "", map[string]any{"client_name": "victim agent"})
	userCode, _ := start.body["user_code"].(string)

	// Someone else saw the code and approves it first with their own account.
	other := f.createUser("code-thief", 1)
	expectAllowed(t, f.do(http.MethodPost, "/api/agent/device/"+userCode+"/approve", "", map[string]any{
		"expired_time": -1, "unlimited_quota": true,
	}, f.login(other.Id)...), "approve with another account")

	resetPollClock(t)
	got := f.agentCall(http.MethodPost, "/api/agent/device/token", ip, "", map[string]any{"device_code": start.body["device_code"]})
	account := agentKeyAccount(t, got)
	if account["type"] != model.AgentKeyAccountPersonal || account["name"] != "code-thief" {
		t.Fatalf("the agent should be told whose key it got: %s", got.raw)
	}
	if _, ok := account["slug"]; ok {
		t.Fatalf("a personal key has no organization slug: %s", got.raw)
	}
}

func TestAgentKeyAnswersNameTheOrganization(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	org, err := model.CreateOrganizationWithOwner("Acme Agents", "", f.admin.Id)
	if err != nil {
		t.Fatal(err)
	}
	cookies := f.login(f.admin.Id)
	base := "/api/org/" + itoa(org.Id)
	ip := "198.51.100.82"
	wantOrg := func(w mkResponse, what string) {
		t.Helper()
		account := agentKeyAccount(t, w)
		if account["type"] != model.AgentKeyAccountOrganization || account["name"] != "Acme Agents" || account["slug"] != org.Slug {
			t.Fatalf("%s: the key should be named as the organization's: %s", what, w.raw)
		}
	}

	start := f.agentCall(http.MethodPost, "/api/agent/device", ip, "", map[string]any{"client_name": "team agent"})
	userCode, _ := start.body["user_code"].(string)
	expectAllowed(t, f.do(http.MethodPost, base+"/agent/device/"+userCode+"/approve", "", map[string]any{
		"expired_time": -1, "unlimited_quota": true,
	}, cookies...), "approve with an organization key")
	resetPollClock(t)
	wantOrg(f.agentCall(http.MethodPost, "/api/agent/device/token", ip, "", map[string]any{"device_code": start.body["device_code"]}), "device flow")

	created := f.do(http.MethodPost, base+"/token", "", map[string]any{"name": "paired", "expired_time": -1, "unlimited_quota": true}, cookies...)
	expectAllowed(t, created, "create an organization key")
	code, _ := created.data()["pairing_code"].(string)
	wantOrg(f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": code}), "pairing code")

	personal := f.do(http.MethodPost, "/api/token/", "", map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true}, cookies...)
	expectAllowed(t, personal, "create a personal key")
	pairCode, _ := personal.data()["pairing_code"].(string)
	paired := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": pairCode})
	if account := agentKeyAccount(t, paired); account["type"] != model.AgentKeyAccountPersonal || account["name"] != f.admin.Username {
		t.Fatalf("a personal paired key should name its owner: %s", paired.raw)
	}
}
