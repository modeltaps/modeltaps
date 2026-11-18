package midjourney

import (
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common/storage"
	"github.com/modeltaps/modeltaps/model"
)

// clientImageURL is the image link a task query answers with: a gateway link (common/media)
// when ImageStorageRehostUpstream is on, so clients download it from this site where the
// Discord or CDN host is blocked or slow. A link on the channel's own host (a Midjourney proxy
// the admin runs, possibly on a private network) and a link that cannot be taken over are passed
// on as they are; the stored task keeps the upstream link either way.
func clientImageURL(task *model.Midjourney) string {
	raw := task.ImageUrl
	if raw == "" || sameHostAsChannel(task.ChannelId, raw) {
		return raw
	}
	link, err := storage.RehostLater(raw, task.ChannelId)
	if err != nil {
		return raw
	}
	return link
}

func sameHostAsChannel(channelID int, raw string) bool {
	if channelID <= 0 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	ch, err := model.GetChannelById(channelID)
	if err != nil || ch.GetBaseURL() == "" {
		return false
	}
	base, err := url.Parse(ch.GetBaseURL())
	return err == nil && base.Host != "" && strings.EqualFold(base.Host, u.Host)
}
