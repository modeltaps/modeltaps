package requesty

import (
	"errors"
	"net/http"

	"github.com/modeltaps/modeltaps/providers/base"
)

// manageBaseURL hosts Requesty's management API, which the balance belongs to.
var manageBaseURL = "https://api-v2.requesty.ai"

// orgResponse is `GET /v1/manage/org`: the organization's prepaid balance in USD.
type orgResponse struct {
	Name    string   `json:"name"`
	Balance *float64 `json:"balance"`
}

// Balance reads the organization's balance with the manage key and stores it on the channel.
// Implements base.BalanceInterface.
func (p *RequestyProvider) Balance() (float64, error) {
	headers := map[string]string{"Authorization": "Bearer " + p.manageKey}
	req, err := p.Requester.NewRequest(http.MethodGet, manageBaseURL+"/v1/manage/org", p.Requester.WithHeader(headers))
	if err != nil {
		return 0, err
	}
	resp := &orgResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		return 0, base.UpstreamReadError(errWithCode, "the Requesty balance needs a key with manage permission: set the channel's management key")
	}
	if resp.Balance == nil {
		return 0, errors.New("Requesty returned no balance")
	}
	p.Channel.UpdateBalance(*resp.Balance)
	return *resp.Balance, nil
}

// CanQueryBalance implements base.BalanceCapability.
func (p *RequestyProvider) CanQueryBalance() bool {
	return true
}
