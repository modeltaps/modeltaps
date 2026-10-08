package common

import (
	"github.com/modeltaps/modeltaps/common/config"
	"fmt"
	"math"
)

func LogQuota(quota int) string {
	if config.DisplayInCurrencyEnabled {
		if quota < 0 {
			return fmt.Sprintf("-$%.6f", math.Abs(float64(quota)/config.QuotaPerUnit))
		}
		return fmt.Sprintf("$%.6f", float64(quota)/config.QuotaPerUnit)
	} else {
		return fmt.Sprintf("%d quota", quota)
	}
}
