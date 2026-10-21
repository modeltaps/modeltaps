package vercel

import (
	"errors"
	"net/http"
	"strconv"
)

// creditsResponse 是 `GET /v1/credits` 的响应：团队余额与累计消耗（USD 字符串），普通 API key 即可查询。
type creditsResponse struct {
	Balance   string `json:"balance"`
	TotalUsed string `json:"total_used"`
}

// Balance 查询团队剩余额度并写入渠道余额。实现 base.BalanceInterface。
func (p *VercelProvider) Balance() (float64, error) {
	fullRequestURL := p.GetFullRequestURL("/v1/credits", "")
	headers := p.GetRequestHeaders()

	req, err := p.Requester.NewRequest(http.MethodGet, fullRequestURL, p.Requester.WithHeader(headers))
	if err != nil {
		return 0, err
	}
	resp := &creditsResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		return 0, errors.New(errWithCode.OpenAIError.Message)
	}

	balance, err := strconv.ParseFloat(resp.Balance, 64)
	if err != nil {
		return 0, errors.New("failed to parse Vercel AI Gateway credits balance")
	}
	p.Channel.UpdateBalance(balance)
	return balance, nil
}
