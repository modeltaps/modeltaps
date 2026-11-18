package model

import (
	"math"
	"strconv"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// How prices work: every request is charged the catalog (official) price times the group ratio,
// whatever channel serves it. The site's margin is the ratio of the default group (1.12 sells at
// 12% over the official price); other groups give discounts or charge more on purpose. What the
// channels cost (their list price with their fee) is the site's own business: it decides which
// supplies take requests and which are held, and never changes what a user pays.

// DefaultUserGroupSymbol is the group every new user starts in.
const DefaultUserGroupSymbol = "default"

// DefaultGroupRatio is the ratio of the default group, 1 when it is missing or not positive.
func DefaultGroupRatio() float64 {
	if g := GlobalUserGroupRatio.GetBySymbol(DefaultUserGroupSymbol); g != nil && g.Ratio > 0 {
		return g.Ratio
	}
	return 1
}

// NetRevenueLine is what one unit of catalog price brings in at the default group's ratio,
// after the cost of taking payments: ratio x (1 - payment cost). A supply whose cost over the
// catalog price is above it loses money on the site's normal price. Discount groups are left
// out on purpose: a discount is the operator's own choice, not a supply's fault.
func NetRevenueLine() float64 {
	return NetRevenueLineAt(DefaultGroupRatio(), config.PaymentCostPercent)
}

// NetRevenueLineAt is NetRevenueLine for another ratio and payment cost (the settings preview).
func NetRevenueLineAt(groupRatio, paymentPercent float64) float64 {
	if groupRatio <= 0 {
		groupRatio = 1
	}
	if paymentPercent < 0 {
		paymentPercent = 0
	}
	return groupRatio * (1 - paymentPercent/100)
}

// foldPlatformMarkupIntoGroups retires the platform markup setting. It used to multiply every
// request on top of the group ratio; the margin now lives in the group ratios themselves, so a
// saved markup is folded into every group's ratio (same charges as before, to the cent the ratio
// column keeps) and the setting is removed, with the cost floor switch that went with it.
func foldPlatformMarkupIntoGroups() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611180901",
		Migrate: func(tx *gorm.DB) error {
			var markup []Option
			if err := tx.Where(&Option{Key: "PlatformMarkupPercent"}).Find(&markup).Error; err != nil {
				return err
			}
			if len(markup) > 0 {
				if m, err := strconv.ParseFloat(markup[0].Value, 64); err == nil && m > 0 && tx.Migrator().HasTable(&UserGroup{}) {
					var groups []UserGroup
					if err := tx.Find(&groups).Error; err != nil {
						return err
					}
					for _, g := range groups {
						if g.Ratio <= 0 {
							continue
						}
						ratio := math.Round(g.Ratio*(1+m/100)*100) / 100
						if err := tx.Model(&UserGroup{}).Where("id = ?", g.Id).Update("ratio", ratio).Error; err != nil {
							return err
						}
						logger.SysLog("platform markup " + strconv.FormatFloat(m, 'f', -1, 64) + "% folded into group " +
							g.Symbol + ": ratio " + strconv.FormatFloat(g.Ratio, 'f', -1, 64) + " -> " + strconv.FormatFloat(ratio, 'f', -1, 64))
					}
				}
			}
			for _, key := range []string{"PlatformMarkupPercent", "SupplyCostFloorBilling"} {
				if err := tx.Where(&Option{Key: key}).Delete(&Option{}).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error { return nil },
	}
}
