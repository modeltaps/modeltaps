package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/model"

	"github.com/spf13/viper"
)

// ReviewPriceSources is the hourly check of the price sources: a channel's upstream price list or
// models.dev that has failed for about a day becomes an alert and one notice; a source that reads
// again closes it. Only sources an automatic sync reads are watched.
func ReviewPriceSources() {
	channels := viper.GetBool("channel_pricing.auto_sync")
	catalog := viper.GetBool("catalog_pricing.auto_sync")
	fresh, err := model.ReviewPriceSources(func(subject string) bool {
		if subject == model.ModelsDevPriceSource {
			// The preset platforms' lists are read from models.dev by the channel sync too.
			return catalog || channels
		}
		return channels
	})
	if err != nil {
		logger.SysError("price source review failed: " + err.Error())
		return
	}
	if len(fresh) == 0 {
		return
	}
	lines := make([]string, 0, len(fresh))
	for _, s := range fresh {
		lines = append(lines, priceSourceNoticeLine(s.PriceSourceState))
	}
	notify.SendKind(notify.KindPriceSource,
		fmt.Sprintf("%d price source(s) stopped updating", len(fresh)),
		"The prices below are kept as they were last read and may be out of date. Check the channel's "+
			"upstream prices tab, or refresh the list there.\n\n"+strings.Join(lines, "\n"))
}

func priceSourceNoticeLine(s *model.PriceSourceState) string {
	name := s.Subject
	if s.Subject == model.ModelsDevPriceSource {
		name = "models.dev"
	} else if id, err := strconv.Atoi(strings.TrimPrefix(s.Subject, "channel:")); err == nil {
		name = fmt.Sprintf("channel #%d", id)
		if ch, err := model.GetChannelById(id); err == nil {
			name = fmt.Sprintf("%s (#%d)", ch.Name, id)
		}
	}
	last := "never"
	if s.LastSuccessAt > 0 {
		last = time.Unix(s.LastSuccessAt, 0).Format("2006-01-02")
	}
	return fmt.Sprintf("- %s: failing since %s, prices from %s: %s", name,
		time.Unix(s.FailingSince, 0).Format("2006-01-02 15:04"), last, s.LastError)
}
