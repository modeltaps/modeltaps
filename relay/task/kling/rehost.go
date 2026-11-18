package kling

import (
	"context"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/storage"
	"github.com/modeltaps/modeltaps/model"
	KlingProvider "github.com/modeltaps/modeltaps/providers/kling"
)

// rehostVideos replaces the video links of a finished task with gateway links (common/media),
// so clients download them from this site where the Kling CDN is blocked or slow. The stored
// task keeps the upstream links; a link that cannot be taken over is passed on.
func rehostVideos(ctx context.Context, task *model.Task, data *KlingProvider.KlingResponse[*KlingProvider.KlingTaskData]) {
	if data == nil || data.Data == nil || data.Data.TaskResult == nil || len(data.Data.TaskResult.Videos) == 0 {
		return
	}
	for i := range data.Data.TaskResult.Videos {
		v := &data.Data.TaskResult.Videos[i]
		if v.URL == "" {
			continue
		}
		link, err := storage.RehostLater(v.URL, task.ChannelId)
		if err != nil {
			if err != storage.ErrStorageOff {
				logger.LogWarn(ctx, "media: passing the Kling video link on: "+err.Error())
			}
			continue
		}
		v.URL = link
	}
}
