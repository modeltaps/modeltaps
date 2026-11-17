package controller

import (
	"fmt"
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// getUserAccountQuota 读取账户级的剩余/已用额度，供 DisplayTokenStatEnabled 关闭时按账户口径返回。
func getUserAccountQuota(c *gin.Context) (remainQuota int, usedQuota int, err error) {
	userData, err := model.GetUserFields(c.GetInt("id"), []string{"quota", "used_quota"})
	if err != nil {
		return 0, 0, err
	}
	return userData["quota"].(int), userData["used_quota"].(int), nil
}

// getOrgKeyQuota returns the figures an organization API key may see on the billing endpoints.
// The key belongs to the organization's shadow account, whose quota and used_quota are the pool
// balance and the whole organization's usage; only owners and admins may see those in the
// console, so a key never reports them, whatever DisplayTokenStatEnabled says (SEC-74):
//   - a key with its own limit reports that limit and its own usage;
//   - an unlimited key reports the period budget of the member who created it, when one is set,
//     the same figures that member sees in the console;
//   - otherwise it reports its own usage and no remaining amount.
func getOrgKeyQuota(orgId int, token *model.Token) (remainQuota int, usedQuota int, expiredTime int64) {
	if !token.UnlimitedQuota {
		return token.RemainQuota, token.UsedQuota, token.ExpiredTime
	}
	if memberId := model.TokenOrgMemberId(token); memberId > 0 {
		if budget, err := model.GetOrgMemberBudget(orgId, memberId); err == nil && budget != nil {
			return max(budget.Budget.Limit-budget.Used, 0), budget.Used, token.ExpiredTime
		}
	}
	return 0, token.UsedQuota, token.ExpiredTime
}

// getBillingQuota resolves the remaining and used quota the billing endpoints report for the
// calling API key.
func getBillingQuota(c *gin.Context) (remainQuota int, usedQuota int, expiredTime int64, err error) {
	if orgId := model.OrganizationIdOfShadowUser(c.GetInt("id")); orgId > 0 {
		token, err := model.GetTokenById(c.GetInt("token_id"))
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get info: %v", err)
		}
		remainQuota, usedQuota, expiredTime = getOrgKeyQuota(orgId, token)
		return remainQuota, usedQuota, expiredTime, nil
	}

	if config.DisplayTokenStatEnabled {
		token, err := model.GetTokenById(c.GetInt("token_id"))
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get info: %v", err)
		}
		if !token.UnlimitedQuota {
			return token.RemainQuota, token.UsedQuota, token.ExpiredTime, nil
		}
	}
	remainQuota, usedQuota, err = getUserAccountQuota(c)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get user info: %v", err)
	}
	return remainQuota, usedQuota, 0, nil
}

func GetSubscription(c *gin.Context) {
	remainQuota, usedQuota, expiredTime, err := getBillingQuota(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if expiredTime <= 0 {
		expiredTime = 0
	}

	quota := remainQuota + usedQuota
	amount := float64(quota)
	if config.DisplayInCurrencyEnabled {
		amount /= config.QuotaPerUnit
	}

	subscription := OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
}

func GetUsage(c *gin.Context) {
	_, quota, _, err := getBillingQuota(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	amount := float64(quota)
	if config.DisplayInCurrencyEnabled {
		amount /= config.QuotaPerUnit
	}
	usage := OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
}
