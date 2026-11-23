package model

import (
	"sort"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// migrateFailoverToTiers retires the failover supply mode. A failover model sent every request
// of a tier to its heaviest supply and kept the lighter ones as backups, which is what a tier
// below does, so the order now says it on its own: one tier, one way to share it (by weight).
//
// Models that routed with failover (their own mode, or the site default SupplyModeDefault when
// they followed it) and kept their own numbers get their tiers split by weight: the heaviest
// supplies of a tier stay in it, each lighter weight moves to a tier of its own right below, in
// the order failover tried them. Such models are set to manual, so the call order does not take
// the split back. Models on the allocation policy already shared their tiers by weight and stay
// as they are. The site default option is removed; the supply_mode column is left unread.
func migrateFailoverToTiers() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611231501",
		Migrate: func(tx *gorm.DB) error {
			siteDefault := "balance"
			if tx.Migrator().HasTable("options") {
				var option Option
				if err := tx.Where(&Option{Key: "SupplyModeDefault"}).Limit(1).Find(&option).Error; err != nil {
					return err
				}
				if option.Value == "failover" {
					siteDefault = "failover"
				}
				if err := tx.Where(&Option{Key: "SupplyModeDefault"}).Delete(&Option{}).Error; err != nil {
					return err
				}
			}
			if !tx.Migrator().HasTable("model_info") || !tx.Migrator().HasColumn("model_info", "supply_mode") ||
				!tx.Migrator().HasTable("model_supplies") {
				return nil
			}
			policyExists := false
			if tx.Migrator().HasTable("allocation_policies") {
				var n int64
				if err := tx.Table("allocation_policies").Where("organization_id = ? AND scope = ?", 0, AllocationScopeDefault).
					Count(&n).Error; err != nil {
					return err
				}
				policyExists = n > 0
			}

			var rows []struct {
				Model      string
				Allocation string
				SupplyMode string
			}
			if err := tx.Table("model_info").Select("model", "allocation", "supply_mode").
				Where("kind = ? AND (alias_of = ? OR alias_of IS NULL)", "", "").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				mode := row.SupplyMode
				if mode != "balance" && mode != "failover" {
					mode = siteDefault
				}
				if mode != "failover" || (policyExists && row.Allocation != AllocationManual) {
					continue
				}
				var supplies []*ModelSupply
				if err := tx.Where("model = ?", row.Model).Find(&supplies).Error; err != nil {
					return err
				}
				numbers, changed := failoverTierNumbers(supplies)
				if !changed {
					continue
				}
				for id, priority := range numbers {
					if err := tx.Model(&ModelSupply{}).Where("id = ?", id).Update("priority", priority).Error; err != nil {
						return err
					}
				}
				if err := tx.Table("model_info").Where("model = ?", row.Model).Update("allocation", AllocationManual).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error { return nil },
	}
}

// failoverTierNumbers renumbers a model's supplies so each tier holds one weight: the priorities
// are tried from the highest, and within a former tier the heavier weights come first. It reports
// false when no tier held two weights, so failover and sharing by weight already routed alike.
func failoverTierNumbers(supplies []*ModelSupply) (map[int]int64, bool) {
	sorted := append([]*ModelSupply(nil), supplies...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		if sorted[i].EffectiveWeight() != sorted[j].EffectiveWeight() {
			return sorted[i].EffectiveWeight() > sorted[j].EffectiveWeight()
		}
		return sorted[i].Id < sorted[j].Id
	})
	var groups [][]*ModelSupply
	changed := false
	for i, s := range sorted {
		if i > 0 {
			prev := sorted[i-1]
			if prev.Priority == s.Priority && prev.EffectiveWeight() == s.EffectiveWeight() {
				groups[len(groups)-1] = append(groups[len(groups)-1], s)
				continue
			}
			if prev.Priority == s.Priority {
				changed = true
			}
		}
		groups = append(groups, []*ModelSupply{s})
	}
	numbers := map[int]int64{}
	for k, group := range groups {
		for _, s := range group {
			numbers[s.Id] = int64(len(groups) - k)
		}
	}
	return numbers, changed
}
