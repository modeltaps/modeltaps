package midjourney

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/spf13/viper"
)

func TestClientImageURL(t *testing.T) {
	setupMJTestDB(t)
	viper.Set("media_dir", t.TempDir())
	saved := []any{config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream}
	config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream = "https://gw.example.com", "local", true
	t.Cleanup(func() {
		viper.Set("media_dir", nil)
		config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream = saved[0].(string), saved[1].(string), saved[2].(bool)
	})
	base := "https://mj-proxy.example.com"
	if err := model.DB.Create(&model.Channel{Id: 1, Name: "mj", BaseURL: &base}).Error; err != nil {
		t.Fatal(err)
	}

	cdn := &model.Midjourney{ChannelId: 1, ImageUrl: "https://cdn.discordapp.com/attachments/1/2/grid.png?ex=1"}
	link := clientImageURL(cdn)
	if !strings.HasPrefix(link, "https://gw.example.com/media/") || !strings.HasSuffix(link, ".png") {
		t.Fatalf("CDN link not moved: %s", link)
	}
	if again := clientImageURL(cdn); again != link {
		t.Fatal("polling again must give the same link")
	}
	own := &model.Midjourney{ChannelId: 1, ImageUrl: "https://mj-proxy.example.com/images/a.png"}
	if got := clientImageURL(own); got != own.ImageUrl {
		t.Fatalf("a link on the channel's own host must be passed on, got %s", got)
	}
	if got := clientImageURL(&model.Midjourney{}); got != "" {
		t.Fatalf("empty: %q", got)
	}
	config.ImageStorageRehostUpstream = false
	if got := clientImageURL(cdn); got != cdn.ImageUrl {
		t.Fatal("rehosted while switched off")
	}
}
