package notify

import (
	"context"
	"fmt"

	"github.com/modeltaps/modeltaps/common/logger"
)

func sendAll(ctx context.Context, title, message string) {
	for _, d := range destinations() {
		if d.Notifier == nil {
			continue
		}
		if err := d.Notifier.Send(ctx, title, message); err != nil {
			logger.LogError(ctx, fmt.Sprintf("notify %s: %s", d.Notifier.Name(), err.Error()))
		}
	}
}

// Send sends a notice to every enabled channel, whatever its kind settings.
func Send(title, message string) {
	//lint:ignore SA1029 reason: 需要使用该类型作为错误处理
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "NotifyTask")
	sendAll(ctx, title, message)
}
