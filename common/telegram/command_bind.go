package telegram

import (
	"github.com/modeltaps/modeltaps/model"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func commandBindInit() (handler ext.Handler) {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("bind", commandBindStart)},
		map[string][]ext.Handler{
			"token": {handlers.NewMessage(noCommands, commandBindToken)},
		},
		cancelConversationOpts(),
	)
}

func commandBindStart(b *gotgbot.Bot, ctx *ext.Context) error {
	user := getBindUser(b, ctx)
	if user != nil {
		ctx.EffectiveMessage.Reply(b, "Your account is already linked, please unlink it first", nil)
		return handlers.EndConversation()
	}

	_, err := ctx.EffectiveMessage.Reply(b, "Please enter your access token", &gotgbot.SendMessageOpts{
		ParseMode:   "html",
		ReplyMarkup: cancelConversationInlineKeyboard(),
	})
	if err != nil {
		return fmt.Errorf("failed to send bind start message: %w", err)
	}
	return handlers.NextConversationState("token")

}

func commandBindToken(b *gotgbot.Bot, ctx *ext.Context) error {
	tgUserId := getTGUserId(b, ctx)
	if tgUserId == 0 {
		return handlers.EndConversation()
	}

	input := ctx.EffectiveMessage.Text
	// 去除input前后空格
	input = strings.TrimSpace(input)

	user := model.ValidateAccessToken(input)
	if user == nil {
		// If the number is not valid, try again!
		ctx.EffectiveMessage.Reply(b, "Invalid token, please try again", &gotgbot.SendMessageOpts{
			ParseMode:   "html",
			ReplyMarkup: cancelConversationInlineKeyboard(),
		})
		// We try the age handler again
		return handlers.NextConversationState("token")
	}

	if user.TelegramId != 0 {
		ctx.EffectiveMessage.Reply(b, "Your account is already linked, please unlink it first", nil)
		return handlers.EndConversation()
	}

	// 查询该tg用户是否已经绑定其他账户
	if model.IsTelegramIdAlreadyTaken(tgUserId) {
		ctx.EffectiveMessage.Reply(b, "This Telegram account is already linked to another account, please unlink it first", nil)
		return handlers.EndConversation()
	}

	// 绑定
	updateUser := model.User{
		Id:         user.Id,
		TelegramId: tgUserId,
	}
	err := updateUser.Update(false)
	if err != nil {
		ctx.EffectiveMessage.Reply(b, "Link failed, please try again later", nil)
		return handlers.EndConversation()
	}

	_, err = ctx.EffectiveMessage.Reply(b, "Linked successfully", nil)
	if err != nil {
		return fmt.Errorf("failed to send bind token message: %w", err)
	}
	return handlers.EndConversation()
}
