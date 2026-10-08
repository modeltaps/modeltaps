package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupLogDetailEndpointDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.LogDetail{}, &model.Log{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

// callRespondLogDetail 直接驱动读取端点鉴权闸门(GetLogDetail/GetOrgLogDetail 均经此),
// 返回 HTTP 状态码与解析后的 data 字段。
func callRespondLogDetail(logId, viewerId, viewerRole int, orgRole string, ownerUserId int) (int, map[string]any) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondLogDetail(c, logId, viewerId, viewerRole, orgRole, ownerUserId)
	var resp struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp.Data
}

// TestRespondLogDetail_DetailPresent 覆盖明细存在时的三档可见性 + 越权 403。
func TestRespondLogDetail_DetailPresent(t *testing.T) {
	setupLogDetailEndpointDB(t)
	const ownerUid, memberUid, otherUid, shadowUid = 1000, 7, 42, 1000
	detail := &model.LogDetail{LogId: 1, TokenId: 5, UserId: ownerUid, CreatedBy: memberUid, RequestBody: "req", ResponseBody: "resp", CreatedAt: time.Now().Unix()}
	if err := model.DB.Create(detail).Error; err != nil {
		t.Fatalf("写入明细失败: %v", err)
	}

	cases := []struct {
		name        string
		viewerId    int
		viewerRole  int
		orgRole     string
		ownerUserId int
		wantCode    int
		wantDetail  bool
	}{
		{"站点管理员全见", 99, config.RoleAdminUser, "", 0, http.StatusOK, true},
		{"组织Owner全见", otherUid, config.RoleCommonUser, model.OrgRoleOwner, shadowUid, http.StatusOK, true},
		{"组织Admin全见", otherUid, config.RoleCommonUser, model.OrgRoleAdmin, shadowUid, http.StatusOK, true},
		{"组织Member-自建组织令牌一律403", memberUid, config.RoleCommonUser, model.OrgRoleMember, shadowUid, http.StatusForbidden, false},
		{"组织Member-他人自建403", otherUid, config.RoleCommonUser, model.OrgRoleMember, shadowUid, http.StatusForbidden, false},
		{"个人-非本人403", otherUid, config.RoleCommonUser, "", 0, http.StatusForbidden, false},
		{"跨组织-归属不符403", otherUid, config.RoleAdminUser, model.OrgRoleOwner, 2000, http.StatusForbidden, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, data := callRespondLogDetail(1, tc.viewerId, tc.viewerRole, tc.orgRole, tc.ownerUserId)
			if code != tc.wantCode {
				t.Fatalf("状态码 = %d, 期望 %d", code, tc.wantCode)
			}
			if tc.wantCode == http.StatusOK {
				if has, _ := data["has_detail"].(bool); has != tc.wantDetail {
					t.Fatalf("has_detail = %v, 期望 %v", data["has_detail"], tc.wantDetail)
				}
				if tc.wantDetail && data["request_body"] != "req" {
					t.Fatalf("request_body 未返回明细: %v", data["request_body"])
				}
			}
		})
	}
}

// TestRespondLogDetail_EmptyState 覆盖无明细时空态原因区分,且空态不绕过鉴权。
func TestRespondLogDetail_EmptyState(t *testing.T) {
	setupLogDetailEndpointDB(t)
	const ownerUid = 1000
	recent := &model.Log{Id: 2, UserId: ownerUid, Type: model.LogTypeConsume, CreatedAt: time.Now().Unix()}
	expired := &model.Log{Id: 3, UserId: ownerUid, Type: model.LogTypeConsume, CreatedAt: time.Now().AddDate(0, 0, -model.LogIORetentionDays-1).Unix()}
	if err := model.DB.Create(recent).Error; err != nil {
		t.Fatalf("写入近期日志失败: %v", err)
	}
	if err := model.DB.Create(expired).Error; err != nil {
		t.Fatalf("写入超期日志失败: %v", err)
	}

	// 近期日志、本人可见 → not_enabled
	code, data := callRespondLogDetail(2, ownerUid, config.RoleCommonUser, "", 0)
	if code != http.StatusOK || data["has_detail"].(bool) || data["reason"] != "not_enabled" {
		t.Fatalf("近期空态应 not_enabled: code=%d data=%v", code, data)
	}
	// 超期日志、本人可见 → expired
	code, data = callRespondLogDetail(3, ownerUid, config.RoleCommonUser, "", 0)
	if code != http.StatusOK || data["reason"] != "expired" {
		t.Fatalf("超期空态应 expired: code=%d data=%v", code, data)
	}
	// 日志不存在 → has_detail=false reason=expired
	code, data = callRespondLogDetail(999, ownerUid, config.RoleCommonUser, "", 0)
	if code != http.StatusOK || data["has_detail"].(bool) {
		t.Fatalf("不存在日志应空态: code=%d data=%v", code, data)
	}
	// 空态不绕过鉴权:个人非本人 → 403
	code, _ = callRespondLogDetail(2, 7, config.RoleCommonUser, "", 0)
	if code != http.StatusForbidden {
		t.Fatalf("空态非本人应 403, 实际 %d", code)
	}
}

// TestCsvSafeField 锁定 CSV 公式注入转义规则(SEC-17)
func TestCsvSafeField(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"=1+1", "'=1+1"},
		{"+1", "'+1"},
		{"-1", "'-1"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
		{"=cmd|'/c calc'!A0", "'=cmd|'/c calc'!A0"},
		{"normal", "normal"},
		{"token-name", "token-name"},
		{"a=1", "a=1"},
		{"用户名", "用户名"},
	}
	for _, tc := range cases {
		if got := csvSafeField(tc.in); got != tc.want {
			t.Fatalf("csvSafeField(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
