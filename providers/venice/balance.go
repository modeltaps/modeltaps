package venice

import (
	"errors"
	"net/http"

	"github.com/modeltaps/modeltaps/providers/base"
)

// balanceResponse is `GET /billing/balance` (beta). Each balance is null when the account has
// none of it. DIEM is the daily inference allowance of staked VVV, one DIEM buying $1 of
// inference; it renews every epoch. usd is net of pending holds and can be negative.
type balanceResponse struct {
	CanConsume          bool    `json:"canConsume"`
	ConsumptionCurrency *string `json:"consumptionCurrency"`
	Balances            struct {
		Diem           *float64 `json:"diem"`
		USD            *float64 `json:"usd"`
		BundledCredits *float64 `json:"bundledCredits"`
		EarnedCredits  *float64 `json:"earnedCredits"`
	} `json:"balances"`
	DiemEpochAllocation float64 `json:"diemEpochAllocation"`
}

// spendable is what the account can still spend, in USD: the USD balance, the DIEM left this
// epoch and any credits, each worth a dollar of inference. An account Venice says cannot consume
// has 0, so the channel is disabled like any channel out of balance.
func (r *balanceResponse) spendable() float64 {
	if !r.CanConsume {
		return 0
	}
	total := 0.0
	for _, v := range []*float64{r.Balances.USD, r.Balances.Diem, r.Balances.BundledCredits, r.Balances.EarnedCredits} {
		if v != nil {
			total += *v
		}
	}
	if total < 0 {
		return 0
	}
	return total
}

// Balance reads the account's balance and stores it on the channel. Implements
// base.BalanceInterface.
func (p *VeniceProvider) Balance() (float64, error) {
	headers := map[string]string{"Authorization": "Bearer " + p.manageKey}
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL("/billing/balance", ""), p.Requester.WithHeader(headers))
	if err != nil {
		return 0, err
	}
	resp := &balanceResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		const refused = "Venice refused the key for the balance: check the channel key, or set a management key that can read billing"
		if errWithCode.StatusCode == http.StatusPaymentRequired {
			return 0, errors.New(refused)
		}
		return 0, base.UpstreamReadError(errWithCode, refused)
	}
	balance := resp.spendable()
	p.Channel.UpdateBalance(balance)
	return balance, nil
}

// CanQueryBalance implements base.BalanceCapability.
func (p *VeniceProvider) CanQueryBalance() bool {
	return true
}
