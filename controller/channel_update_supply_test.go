package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// While supplies route requests, saving a channel attaches it to the public models among the
// names the edit added, and leaves alone a model it listed before whose supply was removed.
func TestUpdateChannelSuppliesAddedModels(t *testing.T) {
	setupBalanceTestDB(t)
	if err := model.DB.AutoMigrate(&model.ModelSupply{}, &model.ModelInfo{}); err != nil {
		t.Fatal(err)
	}
	oldEnabled := config.ModelSupplyEnabled
	config.ModelSupplyEnabled = true
	t.Cleanup(func() { config.ModelSupplyEnabled = oldEnabled })
	gin.SetMode(gin.TestMode)

	for _, name := range []string{"m-old", "m-new"} {
		if err := model.DB.Create(&model.ModelInfo{Model: name, Status: model.ModelStatusPublished}).Error; err != nil {
			t.Fatal(err)
		}
	}
	ch := model.Channel{Name: "c", Type: config.ChannelTypeOpenAI, Key: "sk", Models: "m-old", Group: "default", Status: config.ChannelStatusEnabled}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}

	put := func(models, mapping string) float64 {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		raw, _ := json.Marshal(map[string]any{"id": ch.Id, "type": ch.Type, "name": ch.Name, "models": models,
			"group": "default", "model_mapping": mapping})
		c.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		UpdateChannel(c)
		var got struct {
			Success  bool    `json:"success"`
			Message  string  `json:"message"`
			Supplies float64 `json:"supplies"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || !got.Success {
			t.Fatalf("update failed: %s (%v)", w.Body.String(), err)
		}
		return got.Supplies
	}

	// m-old was listed before and has no supply: the admin removed it, so it stays removed.
	if n := put("m-old,m-new,unknown", `{"m-new":"deploy-new"}`); n != 1 {
		t.Fatalf("supplies = %v, want 1", n)
	}
	var supplies []model.ModelSupply
	model.DB.Where("channel_id = ?", ch.Id).Find(&supplies)
	if len(supplies) != 1 || supplies[0].Model != "m-new" || supplies[0].UpstreamModel != "deploy-new" {
		t.Fatalf("supplies = %+v, want one for m-new sent as deploy-new", supplies)
	}

	// Saving again with the same list adds nothing.
	if n := put("m-old,m-new,unknown", `{"m-new":"deploy-new"}`); n != 0 {
		t.Fatalf("second save supplies = %v, want 0", n)
	}
}

func TestAddedChannelModels(t *testing.T) {
	got := model.AddedChannelModels("a, b", "b,c, a ,d,c")
	if len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Fatalf("added = %v, want [c d]", got)
	}
}
