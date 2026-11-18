package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
)

// Supplies already in use get the same review as new ones (supply_intake_cost.go), as far as it
// is safe. A backup supply above the quote takes requests only when the ones above it fail, so it
// is switched off and waits for review: nothing that serves today stops. A supply that takes
// requests now and costs more than the quote is not switched off, which could take the model
// down; admins are told instead. Only supplies a channel made on its own (or the migration) are
// reviewed, and never once an admin switched them on: CostAcceptedAt records that decision.
// Supplies added by hand are the admin's own.

// supplyAboveQuoteNoticeKey keeps the signature of the serving supplies last reported, so the
// same list is not sent again at every start.
const supplyAboveQuoteNoticeKey = "SupplyAboveQuoteNotice"

// SupplyAboveQuoteReview is what ReviewSuppliesAboveQuote did.
type SupplyAboveQuoteReview struct {
	Held    []*ModelSupplyView // backups switched off, waiting for review
	Serving []*ModelSupplyView // supplies taking requests above the quote, left on
	// Unheld are backups that would be held, left on because the site sells at no margin over the
	// official price yet (NetRevenueLine at most 1): until the default group's ratio gives one,
	// every channel fee counts as a loss and holding them would only take backups away.
	Unheld []*ModelSupplyView
}

// marginConfigured reports whether the site's normal price brings in more than the official
// price after the payment cost: the default group's ratio is set above it.
func marginConfigured() bool {
	return NetRevenueLine() > aboveQuoteSlack
}

func reviewable(v *ModelSupplyView) bool {
	return v.Enabled && !v.Pending && v.CostAcceptedAt == 0 && v.UpstreamGoneAt == 0 &&
		(v.Source == ModelSupplySourceChannel || v.Source == ModelSupplySourceMigrated) &&
		v.Cost != nil && v.Cost.Known && v.Cost.AboveQuote
}

// ReviewSuppliesAboveQuote holds the backup supplies above the quote and reports them, with the
// serving ones, when that list changed since the last report.
func ReviewSuppliesAboveQuote() (*SupplyAboveQuoteReview, error) {
	if !config.ModelSupplyEnabled || DB == nil {
		return &SupplyAboveQuoteReview{}, nil
	}
	views, err := GetModelSupplyViews("", 0)
	if err != nil {
		return nil, err
	}
	out := &SupplyAboveQuoteReview{}
	hold := marginConfigured()
	var holdIds []int
	for _, v := range views {
		if !reviewable(v) {
			continue
		}
		switch v.Traffic {
		case SupplyTrafficBackup:
			if !hold {
				out.Unheld = append(out.Unheld, v)
				continue
			}
			out.Held = append(out.Held, v)
			holdIds = append(holdIds, v.Id)
		case SupplyTrafficPrimary, SupplyTrafficBalanced:
			out.Serving = append(out.Serving, v)
		}
	}
	if len(holdIds) > 0 {
		if err := DB.Model(&ModelSupply{}).Where("id IN ?", holdIds).
			Updates(map[string]any{"enabled": false, "pending": true}).Error; err != nil {
			return nil, err
		}
		reloadSupplies()
	}
	notifyAboveQuoteReview(out)
	return out, nil
}

// ReviewSuppliesAboveQuoteLogged runs the review for a caller that only logs failures.
func ReviewSuppliesAboveQuoteLogged(when string) {
	review, err := ReviewSuppliesAboveQuote()
	if err != nil {
		logger.SysError("supply above-quote review (" + when + ") failed: " + err.Error())
		return
	}
	if len(review.Held) > 0 {
		logger.SysLog(fmt.Sprintf("supply above-quote review (%s): %d backup supplies held for review", when, len(review.Held)))
	}
}

func notifyAboveQuoteReview(r *SupplyAboveQuoteReview) {
	reported := append(append([]*ModelSupplyView{}, r.Serving...), r.Unheld...)
	if len(r.Held) == 0 && !servingChanged(reported) {
		return
	}
	var b strings.Builder
	if len(r.Held) > 0 {
		b.WriteString("These backup supplies cost more than their model's quoted price, so they were switched off " +
			"and wait for review on each model's Supplies tab. They only took requests when the supplies above " +
			"them failed; confirm one to accept its cost, raise the quote first, or keep it off.\n\n")
		b.WriteString(aboveQuoteLines(r.Held))
	}
	if len(r.Serving) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("These supplies take requests and cost more than the quoted price. They were left on, since " +
			"switching them off could leave the model without a supply. With the cost floor on, their requests " +
			"are charged the channel cost; otherwise the difference is lost. Raise the quote, move the supply " +
			"down, or switch it off.\n\n")
		b.WriteString(aboveQuoteLines(r.Serving))
	}
	if len(r.Unheld) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("These backup supplies cost more than what their model's price brings in, but were left on " +
			"because the platform markup has not been set. Set it under Platform settings, Billing; from then " +
			"on such backups wait for review.\n\n")
		b.WriteString(aboveQuoteLines(r.Unheld))
	}
	notify.SendKind(notify.KindSupplyAboveQuote, "Supplies above the quoted price to review", b.String())
}

func aboveQuoteLines(views []*ModelSupplyView) string {
	lines := make([]string, 0, len(views))
	for _, v := range views {
		lines = append(lines, "- "+v.Model+" via "+v.ChannelName+": "+describeSupplyCost(v))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// servingChanged reports whether the serving supplies above the quote differ from the last
// report, and stores the new list. An empty list clears the stored one silently.
func servingChanged(serving []*ModelSupplyView) bool {
	ids := make([]string, 0, len(serving))
	for _, v := range serving {
		ids = append(ids, fmt.Sprint(v.Id))
	}
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, ",")))
	signature := hex.EncodeToString(sum[:8])
	if len(ids) == 0 {
		signature = ""
	}
	option := Option{Key: supplyAboveQuoteNoticeKey}
	if err := DB.FirstOrCreate(&option, Option{Key: supplyAboveQuoteNoticeKey}).Error; err != nil {
		return false
	}
	if option.Value == signature {
		return false
	}
	option.Value = signature
	if err := DB.Save(&option).Error; err != nil {
		return false
	}
	return signature != ""
}

// acceptCostOnSwitchOn records that an admin switched a supply on, so the review leaves it alone.
func acceptCostOnSwitchOn(current *ModelSupply, enabled bool) int64 {
	if current.CostAcceptedAt > 0 {
		return current.CostAcceptedAt
	}
	if enabled && (!current.Enabled || current.Pending) {
		return time.Now().Unix()
	}
	return 0
}
