package openrouter

import (
	"errors"
	"net/http"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
)

// ErrNoKeyLimit 表示 OpenRouter key 没有设置消费上限：`GET /api/v1/key` 只能看到这把 key 的
// 剩余额度（limit_remaining），看不到账户总余额。管理员需在 OpenRouter 后台给网关用的 key
// 设一个上限，余额追踪才有数可读；配额快照（用量部分）不受影响，照常写入。
var ErrNoKeyLimit = errors.New("OpenRouter key has no spend limit; set one in the OpenRouter dashboard, or give the channel a management key so the account balance can be read")

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

// creditsResponse is `GET /api/v1/credits`, which only a management key may call: what the
// account bought and what it used, in USD. The account balance is the difference.
// https://openrouter.ai/docs/api/api-reference/credits/get-credits
type creditsResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

// Balance stores what the channel can still spend and the quota snapshot. Implements
// base.BalanceInterface. The inference key reports its own limit; with a management key on the
// channel the account balance is read too, and the smaller of the two is the channel's balance
// (a key cannot spend more than its limit, nor more than the account holds).
func (p *OpenRouterProvider) Balance() (float64, error) {
	info, err := p.fetchKeyInfo()
	if err != nil {
		return 0, err
	}
	snapshot := snapshotFromKeyInfo(info, utils.GetTimestamp())

	var accountErr error
	if manageKey := p.Channel.GetManageKey(); manageKey != "" {
		var account float64
		if account, accountErr = p.fetchAccountBalance(manageKey); accountErr == nil {
			snapshot.AccountBalance = &account
		}
	}
	_ = p.Channel.UpdateQuotaSnapshot(snapshot)

	var balance float64
	switch {
	case info.LimitRemaining != nil && snapshot.AccountBalance != nil:
		balance = min(*info.LimitRemaining, *snapshot.AccountBalance)
	case info.LimitRemaining != nil:
		balance = *info.LimitRemaining
	case snapshot.AccountBalance != nil:
		balance = *snapshot.AccountBalance
	case accountErr != nil:
		return 0, accountErr
	default:
		return 0, ErrNoKeyLimit
	}
	p.Channel.UpdateBalance(balance)
	return balance, nil
}

// fetchAccountBalance reads the account's remaining credits with the management key.
func (p *OpenRouterProvider) fetchAccountBalance(manageKey string) (float64, error) {
	headers := p.GetRequestHeaders()
	headers["Authorization"] = "Bearer " + manageKey
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL("/v1/credits", ""), p.Requester.WithHeader(headers))
	if err != nil {
		return 0, err
	}
	resp := &creditsResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		return 0, base.UpstreamReadError(errWithCode, "the OpenRouter account balance needs a management key: check the channel's management key")
	}
	return resp.Data.TotalCredits - resp.Data.TotalUsage, nil
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
		return nil, base.UpstreamReadError(errWithCode, "")
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

// CanQueryBalance implements base.BalanceCapability: this provider has a balance endpoint.
func (p *OpenRouterProvider) CanQueryBalance() bool {
	return true
}
