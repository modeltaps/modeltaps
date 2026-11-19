package cheaperinference

import (
	"errors"
	"net/http"
)

// balanceResponse is `GET /v1/account/balance`: the wallet in USD. available_usd is the balance
// less what is held for requests in flight, and is what can be spent.
type balanceResponse struct {
	BalanceUSD   *float64 `json:"balance_usd"`
	AvailableUSD *float64 `json:"available_usd"`
	ReservedUSD  float64  `json:"reserved_usd"`
}

// Balance reads the spendable wallet balance with the manage key and stores it on the channel.
// Implements base.BalanceInterface.
func (p *CheaperInferenceProvider) Balance() (float64, error) {
	headers := map[string]string{"Authorization": "Bearer " + p.manageKey}
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL("/v1/account/balance", ""), p.Requester.WithHeader(headers))
	if err != nil {
		return 0, err
	}
	resp := &balanceResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		switch errWithCode.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return 0, errors.New("the Cheaper Inference balance needs a key with the account:read scope: set the channel's management key")
		}
		return 0, errors.New(errWithCode.OpenAIError.Message)
	}
	balance, ok := resp.spendable()
	if !ok {
		return 0, errors.New("Cheaper Inference returned no balance")
	}
	p.Channel.UpdateBalance(balance)
	return balance, nil
}

func (r *balanceResponse) spendable() (float64, bool) {
	v := r.AvailableUSD
	if v == nil {
		v = r.BalanceUSD
	}
	if v == nil {
		return 0, false
	}
	if *v < 0 {
		return 0, true
	}
	return *v, true
}

// CanQueryBalance implements base.BalanceCapability.
func (p *CheaperInferenceProvider) CanQueryBalance() bool {
	return true
}
