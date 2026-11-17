package relay

import (
	"encoding/json"
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/safty"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

// checkContentSafety runs content moderation over every text block in parts (messages, system
// prompts, tools, inputs, prompts) as one check. It returns nil when moderation is off or the
// content passes, and the local 400 error to send the client when the content is refused. Every
// entry calls it before any upstream request.
func checkContentSafety(parts ...any) *types.OpenAIErrorWithStatusCode {
	if !config.EnableSafe {
		return nil
	}
	result, _ := safty.CheckContent(parts)
	if result.IsSafe {
		return nil
	}
	return common.StringErrorWrapperLocal(result.Reason, result.Code, http.StatusBadRequest)
}

// geminiModeratedFields are the top-level members of a Gemini generateContent body that carry
// text the model reads.
var geminiModeratedFields = map[string]bool{
	"contents":           true,
	"systemInstruction":  true,
	"system_instruction": true,
	"tools":              true,
}

// geminiContentSafety moderates a raw Gemini generateContent body: the text parts of contents,
// the function calls and responses and code parts in them (every string, whatever its key), the
// system instruction, and the tool declarations (their descriptions). The bytes are read with
// gjson so inline media is never decoded, and every occurrence of a duplicated top-level key is
// checked, since the upstream may read a different one than the gateway does.
func geminiContentSafety(body []byte) *types.OpenAIErrorWithStatusCode {
	if !config.EnableSafe || body == nil {
		return nil
	}

	texts := make([]string, 0)
	gjson.ParseBytes(body).ForEach(func(key, value gjson.Result) bool {
		if geminiModeratedFields[key.String()] {
			texts = append(texts, safty.ExtractJSONText(value.Raw)...)
		}
		return true
	})
	return checkContentSafety(texts)
}

// realtimeClientEventSafety moderates a client event of a Realtime session before it is
// forwarded upstream: the item of conversation.item.create, the session of session.update
// (instructions, tool descriptions) and the response of response.create (instructions, input
// items). Other events (audio buffers and the like) carry no text and pass.
func realtimeClientEventSafety(message []byte) *types.OpenAIErrorWithStatusCode {
	if !config.EnableSafe {
		return nil
	}

	var field string
	switch gjson.GetBytes(message, "type").String() {
	case "conversation.item.create":
		field = "item"
	case "session.update":
		field = "session"
	case "response.create":
		field = "response"
	default:
		return nil
	}
	value := gjson.GetBytes(message, field)
	if !value.Exists() || value.Raw == "" {
		return nil
	}
	return checkContentSafety(json.RawMessage(value.Raw))
}

// moderateRealtimeClientEvents wraps a Realtime message handler so client text events are
// moderated before they are forwarded. A refused event is not forwarded: the client gets an
// error event through replyToUser and the session ends.
func moderateRealtimeClientEvents(next requester.MessageHandler, replyToUser func(data []byte) error) requester.MessageHandler {
	return func(source requester.MessageSource, messageType int, message []byte) (bool, *types.UsageEvent, []byte, error) {
		if source == requester.UserMessage && messageType == websocket.TextMessage {
			if apiErr := realtimeClientEventSafety(message); apiErr != nil {
				code, _ := apiErr.OpenAIError.Code.(string)
				event := types.NewErrorEvent("", "invalid_request_error", code, apiErr.OpenAIError.Message)
				if replyToUser != nil {
					_ = replyToUser([]byte(event.Error()))
				}
				return false, nil, nil, nil
			}
		}
		if next == nil {
			return true, nil, nil, nil
		}
		return next(source, messageType, message)
	}
}
