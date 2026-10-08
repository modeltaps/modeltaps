package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupAddRedemptionTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Redemption{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

func callAddRedemption(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/redemption/", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 1)
	AddRedemption(c)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

func countRedemptions(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := model.DB.Model(&model.Redemption{}).Count(&n).Error; err != nil {
		t.Fatalf("统计兑换码失败: %v", err)
	}
	return n
}

// Quota 必须 > 0:0 与 -1 应被拒绝且不落库;正常额度应成功创建。
func TestAddRedemptionQuotaValidation(t *testing.T) {
	t.Run("Quota=0 被拒不落库", func(t *testing.T) {
		setupAddRedemptionTestDB(t)
		resp := callAddRedemption(t, `{"name":"q0","count":1,"quota":0}`)
		if resp["success"] != false {
			t.Fatalf("Quota=0 应被拒,实际: %v", resp)
		}
		if resp["message"] != "Redemption code quota must be greater than 0" {
			t.Fatalf("错误文案不符,实际: %v", resp["message"])
		}
		if got := countRedemptions(t); got != 0 {
			t.Fatalf("被拒不应落库,实际 %d", got)
		}
	})

	t.Run("Quota=-1 被拒不落库", func(t *testing.T) {
		setupAddRedemptionTestDB(t)
		resp := callAddRedemption(t, `{"name":"qneg","count":1,"quota":-1}`)
		if resp["success"] != false {
			t.Fatalf("Quota=-1 应被拒,实际: %v", resp)
		}
		if got := countRedemptions(t); got != 0 {
			t.Fatalf("被拒不应落库,实际 %d", got)
		}
	})

	t.Run("Quota>0 正常创建", func(t *testing.T) {
		setupAddRedemptionTestDB(t)
		resp := callAddRedemption(t, `{"name":"qok","count":3,"quota":100}`)
		if resp["success"] != true {
			t.Fatalf("Quota>0 应成功,实际: %v", resp)
		}
		data, ok := resp["data"].([]interface{})
		if !ok || len(data) != 3 {
			t.Fatalf("应返回 3 个 key,实际: %v", resp["data"])
		}
		if got := countRedemptions(t); got != 3 {
			t.Fatalf("落库数量应与返回 key 一致(3),实际 %d", got)
		}
	})
}
