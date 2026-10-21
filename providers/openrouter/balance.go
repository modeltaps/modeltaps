package openrouter

import (
	"errors"
	"net/http"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

// ErrNoKeyLimit 表示 OpenRouter key 没有设置消费上限：`GET /api/v1/key` 只能看到这把 key 的
// 剩余额度（limit_remaining），看不到账户总余额。管理员需在 OpenRouter 后台给网关用的 key
// 设一个上限，余额追踪才有数可读；配额快照（用量部分）不受影响，照常写入。
var ErrNoKeyLimit = errors.New("OpenRouter key has no spend limit; set one in the OpenRouter dashboard so the remaining amount can be tracked")

// keyInfoResponse 是 `GET /api/v1/key` 的响应（用普通推理 key 鉴权即可，不需要 Management Key）。
type keyInfoResponse struct {
	Data keyInfo `json:"data"`
}

type keyInfo struct {
	Label          string   `json:"label"`
	Limit          *float64 `json:"limit"`
	LimitRemaining *float64 `json:"limit_remaining"`
	LimitReset     *string  `json:"limit_reset"`
	Usage          float64  `json:"usage"`
	UsageDaily     float64  `json:"usage_daily"`
	UsageWeekly    float64  `json:"usage_weekly"`
	UsageMonthly   float64  `json:"usage_monthly"`
	IsFreeTier     bool     `json:"is_free_tier"`
}

// Balance 查询当前 key 的剩余额度并写入渠道余额与配额快照。实现 base.BalanceInterface。
func (p *OpenRouterProvider) Balance() (float64, error) {
	info, err := p.fetchKeyInfo()
	if err != nil {
		return 0, err
	}

	snapshot := snapshotFromKeyInfo(info, utils.GetTimestamp())
	_ = p.Channel.UpdateQuotaSnapshot(snapshot)

	if info.LimitRemaining == nil {
		return 0, ErrNoKeyLimit
	}
	balance := *info.LimitRemaining
	p.Channel.UpdateBalance(balance)
	return balance, nil
}

func (p *OpenRouterProvider) fetchKeyInfo() (*keyInfo, error) {
	fullRequestURL := p.GetFullRequestURL("/v1/key", "")
	headers := p.GetRequestHeaders()

	req, err := p.Requester.NewRequest(http.MethodGet, fullRequestURL, p.Requester.WithHeader(headers))
	if err != nil {
		return nil, err
	}
	resp := &keyInfoResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		return nil, errors.New(errWithCode.OpenAIError.Message)
	}
	return &resp.Data, nil
}

// snapshotFromKeyInfo 把 key 信息整理成渠道配额快照。
func snapshotFromKeyInfo(info *keyInfo, fetchedAt int64) model.ChannelQuotaSnapshot {
	snapshot := model.ChannelQuotaSnapshot{
		Limit:        info.Limit,
		Remaining:    info.LimitRemaining,
		Usage:        info.Usage,
		UsageDaily:   info.UsageDaily,
		UsageWeekly:  info.UsageWeekly,
		UsageMonthly: info.UsageMonthly,
		IsFreeTier:   info.IsFreeTier,
		FetchedAt:    fetchedAt,
	}
	if info.LimitReset != nil {
		snapshot.ResetInterval = *info.LimitReset
	}
	return snapshot
}
