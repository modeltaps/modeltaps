package telegram

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"fmt"
	"net/url"
	"strings"

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

	chatUrlTmp := ""
	if config.ServerAddress != "" {
		chatUrlTmp = getChatUrl()
	}

	message = "Tap an API key to copy it:\n"

	for _, token := range *list.Data {
		key := "sk-" + token.Key
		message += fmt.Sprintf("*%s* : `%s`\n", escapeText(token.Name, "MarkdownV2"), key)
		if chatUrlTmp != "" {
			message += strings.ReplaceAll(chatUrlTmp, `setToken`, key)
		}
		message += "\n"
	}

	return message, getPageParams("apikey", page, genericParams.Size, int(list.TotalCount))
}

func getChatUrl() string {
	serverAddress := strings.TrimSuffix(config.ServerAddress, "/")
	chatNextUrl := fmt.Sprintf(`{"key":"setToken","url":"%s"}`, serverAddress)
	if config.ChatLink != "" {
		chatLink := strings.TrimSuffix(config.ChatLink, "/")
		chatNextUrl = chatLink + "/#/?settings=" + url.QueryEscape(chatNextUrl)
	} else {
		chatNextUrl = ""
	}

	jumpUrl := fmt.Sprintf(`%s/jump?url=`, serverAddress)

	amaUrl := jumpUrl + url.QueryEscape(fmt.Sprintf(`ama://set-api-key?server=%s&key=setToken`, serverAddress))

	openCatUrl := jumpUrl + url.QueryEscape(fmt.Sprintf(`opencat://team/join?domain=%s&token=setToken`, serverAddress))

	return fmt.Sprintf("[Next Chat](%s)  [AMA](%s)  [OpenCat](%s)\n", chatNextUrl, amaUrl, openCatUrl)
}
