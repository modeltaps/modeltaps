package kling

import (
	"context"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	KlingProvider "github.com/modeltaps/modeltaps/providers/kling"
	"github.com/spf13/viper"
)

func TestRehostVideos(t *testing.T) {
	viper.Set("media_dir", t.TempDir())
	saved := []any{config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream}
	config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream = "https://gw.example.com", "local", true
	t.Cleanup(func() {
		viper.Set("media_dir", nil)
		config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream = saved[0].(string), saved[1].(string), saved[2].(bool)
	})

	data := func() *KlingProvider.KlingResponse[*KlingProvider.KlingTaskData] {
		return &KlingProvider.KlingResponse[*KlingProvider.KlingTaskData]{Data: &KlingProvider.KlingTaskData{
			TaskStatus: "succeed",
			TaskResult: &KlingProvider.KlingTaskResult{Videos: []KlingProvider.KlingVideoResult{
				{ID: "1", URL: "https://cdn.klingai.com/out/1.mp4"},
				{ID: "2", URL: ""},
			}},
		}}
	}
	task := &model.Task{ChannelId: 3}

	got := data()
	rehostVideos(context.Background(), task, got)
	first := got.Data.TaskResult.Videos[0].URL
	if !strings.HasPrefix(first, "https://gw.example.com/media/") || !strings.HasSuffix(first, ".mp4") {
		t.Fatalf("video link not moved: %s", first)
	}
	if got.Data.TaskResult.Videos[1].URL != "" {
		t.Fatal("an empty link was filled")
	}
	again := data()
	rehostVideos(context.Background(), task, again)
	if again.Data.TaskResult.Videos[0].URL != first {
		t.Fatal("polling the task again must give the same link")
	}

	config.ImageStorageRehostUpstream = false
	off := data()
	rehostVideos(context.Background(), task, off)
	if off.Data.TaskResult.Videos[0].URL != "https://cdn.klingai.com/out/1.mp4" {
		t.Fatal("rehosted while switched off")
	}
	rehostVideos(context.Background(), task, nil)
}
