package controller

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// /api/option/ 不下发密钥原值：data 里没有密钥项，secrets 给出全部密钥项的回显(未设置为空串)。
func TestGetOptionsReturnsMaskedSecrets(t *testing.T) {
	setupOidcTestDB(t)
	model.InitOptionMap()

	const smtpToken = "smtp-token-0123456789abcdef"
	prevSMTP, prevTurnstile := config.SMTPToken, config.TurnstileSecretKey
	config.SMTPToken, config.TurnstileSecretKey = smtpToken, ""
	t.Cleanup(func() { config.SMTPToken, config.TurnstileSecretKey = prevSMTP, prevTurnstile })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/option/", nil)
	GetOptions(c)

	if strings.Contains(recorder.Body.String(), smtpToken) {
		t.Fatalf("响应泄露 SMTPToken 原值: %s", recorder.Body.String())
	}
	var body struct {
		Data    []model.Option    `json:"data"`
		Secrets map[string]string `json:"secrets"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 /api/option/ 失败: %v", err)
	}
	for _, option := range body.Data {
		if _, isSecret := body.Secrets[option.Key]; isSecret {
			t.Errorf("密钥项 %s 不应出现在 data 里", option.Key)
		}
	}
	if got := body.Secrets["SMTPToken"]; got != "smtp****cdef" {
		t.Errorf("SMTPToken 回显 = %q, want smtp****cdef", got)
	}
	if got, ok := body.Secrets["TurnstileSecretKey"]; !ok || got != "" {
		t.Errorf("未设置的 TurnstileSecretKey 应以空串出现, got %q (present=%v)", got, ok)
	}
}
