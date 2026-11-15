package notify

import (
	"context"
	"fmt"

	"github.com/modeltaps/modeltaps/common/logger"
)

func sendAll(ctx context.Context, kind, title, message string) {
	ctx = WithKind(ctx, kind)
	for _, d := range destinations() {
		if d.Notifier == nil || !d.Wants(kind) {
			continue
		}
		err := d.Notifier.Send(ctx, title, message)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("notify %s: %s", d.Notifier.Name(), err.Error()))
		}
		Record(Delivery{Kind: kind, Type: d.Type, Name: d.Name, Title: title, Err: err})
	}
}

// Record passes a delivery to the send record, if one is set.
func Record(d Delivery) {
	if OnDelivered != nil {
		OnDelivered(d)
	}
}

// Send sends a notice to every enabled channel, whatever its kind settings.
func Send(title, message string) {
	//lint:ignore SA1029 reason: 需要使用该类型作为错误处理
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "NotifyTask")
	sendAll(ctx, "", title, message)
}
