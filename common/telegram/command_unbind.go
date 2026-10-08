package telegram

import (
	"github.com/modeltaps/modeltaps/model"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func commandUnbindStart(b *gotgbot.Bot, ctx *ext.Context) error {
	user := getBindUser(b, ctx)
	if user == nil {
		return nil
	}

	updateUser := map[string]interface{}{
		"telegram_id": 0,
	}

	err := model.UpdateUser(user.Id, updateUser)
	if err != nil {
		ctx.EffectiveMessage.Reply(b, "Unlink failed, please try again later", nil)
		return handlers.EndConversation()
	}

	ctx.EffectiveMessage.Reply(b, "Unlinked successfully", nil)
	return nil
}
