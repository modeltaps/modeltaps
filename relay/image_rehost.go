package relay

import (
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/storage"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// rehostUpstreamImages moves images the upstream returned as links into the image storage
// (ImageStorageRehostUpstream), so the client gets a link on the site's own address. They are
// fetched through the channel's proxy, at once for an n>1 response; a link that cannot be
// fetched is passed on as it is. When any link changed, the stashed upstream bytes are dropped
// so the response is written from the struct.
func rehostUpstreamImages(c *gin.Context, provider providersBase.ProviderInterface, resp *types.ImageResponse) {
	if !config.ImageStorageRehostUpstream || resp == nil || len(resp.Data) == 0 {
		return
	}
	own := strings.TrimRight(strings.TrimSpace(config.ServerAddress), "/") + "/"
	proxy := ""
	if provider != nil {
		if ch := provider.GetChannel(); ch != nil && ch.Proxy != nil {
			proxy = *ch.Proxy
		}
	}
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		changed bool
	)
	for i := range resp.Data {
		item := &resp.Data[i]
		if item.URL == "" || item.B64JSON != "" || strings.HasPrefix(item.URL, own) ||
			!(strings.HasPrefix(item.URL, "https://") || strings.HasPrefix(item.URL, "http://")) {
			continue
		}
		wg.Add(1)
		go func(item *types.ImageResponseDataInner) {
			defer wg.Done()
			link, err := storage.Rehost(c.Request.Context(), item.URL, proxy)
			if err != nil {
				logger.LogWarn(c.Request.Context(), "image storage: passing the upstream image link on: "+err.Error())
				return
			}
			mu.Lock()
			item.URL, changed = link, true
			mu.Unlock()
		}(item)
	}
	wg.Wait()
	if changed {
		c.Set(config.GinRawResponseBodyKey, []byte(nil))
	}
}
