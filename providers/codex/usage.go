package codex

import (
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

// codexUsageWindow and codexUsageResponse follow /backend-api/wham/usage, the endpoint the
// Codex CLI reads its plan limits from.
type codexUsageWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	ResetAt            int64   `json:"reset_at"`
	ResetAfterSeconds  int64   `json:"reset_after_seconds"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
}

type codexUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		Allowed         *bool             `json:"allowed"`
		LimitReached    bool              `json:"limit_reached"`
		PlanType        string            `json:"plan_type"`
		PrimaryWindow   *codexUsageWindow `json:"primary_window"`
		SecondaryWindow *codexUsageWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

// FetchUsageWindows implements base.UsageWindowsInterface: it queries the account's 5-hour
// and weekly usage windows.
func (p *CodexProvider) FetchUsageWindows() (*model.ChannelUsageWindows, error) {
	headers, err := p.getRequestHeadersInternal()
	if err != nil {
		return nil, err
	}
	delete(headers, "Content-Type")
	headers["Accept"] = "application/json"
	if _, ok := headers["originator"]; !ok {
		headers["originator"] = DefaultCodexOriginator
	}
	url := strings.TrimRight(p.GetBaseURL(), "/") + "/backend-api/wham/usage"
	req, err := p.Requester.NewRequest(http.MethodGet, url, p.Requester.WithHeader(headers))
	if err != nil {
		return nil, err
	}
	resp := &codexUsageResponse{}
	if _, errWithCode := p.Requester.SendRequest(req, resp, false); errWithCode != nil {
		return nil, errWithCode
	}
	return resp.toUsageWindows(), nil
}

func (r *codexUsageResponse) toUsageWindows() *model.ChannelUsageWindows {
	out := &model.ChannelUsageWindows{Source: "api", FetchedAt: utils.GetTimestamp(), PlanType: r.PlanType}
	if out.PlanType == "" {
		out.PlanType = r.RateLimit.PlanType
	}
	switch {
	case r.RateLimit.LimitReached:
		out.Status = "rejected"
	case r.RateLimit.Allowed != nil && *r.RateLimit.Allowed:
		out.Status = "allowed"
	}
	now := utils.GetTimestamp()
	for _, w := range []*codexUsageWindow{r.RateLimit.PrimaryWindow, r.RateLimit.SecondaryWindow} {
		if w == nil {
			continue
		}
		resetAt := w.ResetAt
		if resetAt == 0 && w.ResetAfterSeconds > 0 {
			resetAt = now + w.ResetAfterSeconds
		}
		out.Windows = append(out.Windows, model.UsageWindow{
			Name:          model.WindowName(w.LimitWindowSeconds),
			UsedPercent:   w.UsedPercent,
			ResetAt:       resetAt,
			WindowSeconds: w.LimitWindowSeconds,
		})
	}
	return out
}
