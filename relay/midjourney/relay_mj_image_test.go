package midjourney

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupMJTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "mj.db")
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.Midjourney{}, &model.Channel{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite := model.DB, common.UsingSQLite
	model.DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { model.DB, common.UsingSQLite = oldDB, oldSQLite })
}

func serveMJImage(t *testing.T, mjID string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/mj/image/"+mjID, nil)
	c.Params = gin.Params{{Key: "id", Value: mjID}}
	RelayMidjourneyImage(c)
	return w
}

func TestRelayMidjourneyImageBlocksPrivateURLs(t *testing.T) {
	setupMJTestDB(t)
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("secret"))
	}))
	defer internal.Close()

	other := "https://mj.example.com"
	if err := model.DB.Create(&model.Channel{Id: 1, Name: "mj", BaseURL: &other}).Error; err != nil {
		t.Fatal(err)
	}
	tasks := []model.Midjourney{
		{MjId: "private", ChannelId: 1, ImageUrl: internal.URL + "/a.png"},
		{MjId: "metadata", ChannelId: 1, ImageUrl: "http://169.254.169.254/latest/meta-data/"},
		{MjId: "file", ChannelId: 1, ImageUrl: "file:///etc/passwd"},
	}
	if err := model.DB.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		w := serveMJImage(t, task.MjId)
		if w.Code == http.StatusOK || w.Body.String() == "secret" {
			t.Fatalf("task %s: private image url must not be fetched, got %d %q", task.MjId, w.Code, w.Body.String())
		}
	}
}

func TestRelayMidjourneyImageAllowsChannelHost(t *testing.T) {
	setupMJTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<script>alert(1)</script>"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer upstream.Close()

	base := upstream.URL
	if err := model.DB.Create(&model.Channel{Id: 2, Name: "mj", BaseURL: &base}).Error; err != nil {
		t.Fatal(err)
	}
	tasks := []model.Midjourney{
		{MjId: "ok", ChannelId: 2, ImageUrl: upstream.URL + "/attachments/a.png"},
		{MjId: "html", ChannelId: 2, ImageUrl: upstream.URL + "/page"},
	}
	if err := model.DB.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if w := serveMJImage(t, "ok"); w.Code != http.StatusOK || w.Body.String() != "png" {
		t.Fatalf("image on the channel host should be served, got %d %q", w.Code, w.Body.String())
	}
	if w := serveMJImage(t, "html"); w.Code == http.StatusOK {
		t.Fatalf("non-image content must not be relayed, got %d", w.Code)
	}
}

func TestRelayMidjourneyNotifyRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/notify", nil)
	if resp := RelayMidjourneyNotify(c); resp == nil || resp.Code != 4 {
		t.Fatalf("notify must be rejected, got %+v", resp)
	}
}
