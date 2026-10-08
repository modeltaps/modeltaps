package telegram

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func commandAffStart(b *gotgbot.Bot, ctx *ext.Context) error {
	user := getBindUser(b, ctx)
	if user == nil {
		return nil
	}

	if user.AffCode == "" {
		user.AffCode = utils.GetRandomString(4)
		if err := user.Update(false); err != nil {
			ctx.EffectiveMessage.Reply(b, "System error, please try again later", nil)
			return nil
		}
	}

	messae := "Share your invite code to invite friends. You earn a reward for every successful invitation.\n\nYour invite code: " + user.AffCode
	if config.ServerAddress != "" {
		serverAddress := strings.TrimSuffix(config.ServerAddress, "/")
		messae += "\n\nPage URL: " + serverAddress + "/register?aff=" + user.AffCode
	}

	ctx.EffectiveMessage.Reply(b, messae, nil)

	return nil
}
