package telegram

import (
	"github.com/modeltaps/modeltaps/model"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func commandApikeyStart(b *gotgbot.Bot, ctx *ext.Context) error {
	user := getBindUser(b, ctx)
	if user == nil {
		return nil
	}

	message, pageParams := getApikeyList(user.Id, 1)
	if pageParams == nil {
		_, err := ctx.EffectiveMessage.Reply(b, message, nil)
		if err != nil {
			return fmt.Errorf("failed to send APIKEY message: %w", err)
		}
		return nil
	}

	_, err := ctx.EffectiveMessage.Reply(b, message, &gotgbot.SendMessageOpts{
		ParseMode:   "MarkdownV2",
		ReplyMarkup: getPaginationInlineKeyboard(pageParams.key, pageParams.page, pageParams.total),
	})
	if err != nil {
		return fmt.Errorf("failed to send APIKEY message: %w", err)
	}

	return nil
}

func getApikeyList(userId, page int) (message string, pageParams *paginationParams) {
	genericParams := &model.GenericParams{
		PaginationParams: model.PaginationParams{
			Page: page,
			Size: 5,
		},
	}

	list, err := model.GetUserTokensList(userId, genericParams)

	if err != nil {
		return "System error, please try again later", nil
	}

	if list.Data == nil || len(*list.Data) == 0 {
		return "No API key found", nil
	}

	// Keys are stored as hashes only (SEC-47): the bot lists the masked form, and a key is only
	// ever shown in the console, once, when it is created or reset.
	message = escapeText("Your API keys (masked). A key is only shown once, in the console, when it is created or reset.", "MarkdownV2") + "\n\n"

	for _, token := range *list.Data {
		message += fmt.Sprintf("*%s* : `%s`\n\n", escapeText(token.Name, "MarkdownV2"), token.KeyHint)
	}

	return message, getPageParams("apikey", page, genericParams.Size, int(list.TotalCount))
}
