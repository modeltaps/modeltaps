package model

import (
	"encoding/json"
	"slices"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/modeltaps/modeltaps/common/config"
)

// ChannelDriftImpact is what the models the upstream no longer lists mean for what the site
// serves, worked out when the channel is read (the last model check says which are missing; the
// supplies and public models say whether that matters now).
type ChannelDriftImpact struct {
	// Serving are the flagged missing models (channel-side names) a public model is served
	// through: while supplies route requests, an enabled, confirmed supply of a published public
	// model; otherwise the listed name itself. Missing models nothing is served through are left
	// out: their going away changes nothing for users.
	Serving []string `json:"serving"`
	// Renamed maps a flagged missing model to the name the upstream now lists for the same
	// public model ("gpt-5.5" → "openai/gpt-5.5"): switching to it restores the model.
	Renamed map[string]string `json:"renamed,omitempty"`
}

// flaggedMissing is the missing models of the channel's last check an admin has not handled:
// still on its list, neither dismissed nor switched off.
func (c *Channel) flaggedMissing() []string {
	drift := c.GetModelDrift()
	if drift == nil || len(drift.MissingModels) == 0 {
		return nil
	}
	listed := map[string]bool{}
	for _, m := range SplitChannelModels(c.Models) {
		listed[m] = true
	}
	disabled := c.DisabledModelSet()
	var out []string
	for _, m := range drift.MissingModels {
		if listed[m] && !disabled[m] && !slices.Contains(drift.DismissedMissing, m) {
			out = append(out, m)
		}
	}
	return out
}

// upstreamNameOf is the name a listed model is sent upstream as, after the model mapping (see
// channelMappingOf).
func upstreamNameOf(listed string, mapping map[string]string) string {
	if mapped := mapping[listed]; mapped != "" {
		return mapped
	}
	return listed
}

// DriftImpacts works out the drift impact of the channels with flagged missing models, keyed
// by channel id; channels without any are left out.
func DriftImpacts(channels []*Channel) (map[int]*ChannelDriftImpact, error) {
	flagged := map[int][]string{}
	var ids []int
	var names []string
	for _, c := range channels {
		missing := c.flaggedMissing()
		if len(missing) == 0 {
			continue
		}
		flagged[c.Id] = missing
		ids = append(ids, c.Id)
		mapping := channelMappingOf(c)
		for _, m := range missing {
			names = append(names, m, upstreamNameOf(m, mapping))
		}
	}
	if len(flagged) == 0 {
		return map[int]*ChannelDriftImpact{}, nil
	}

	// Supplies of the flagged names, and the public state of the models they serve.
	supplies := map[int][]*ModelSupply{}
	published := map[string]bool{}
	if config.ModelSupplyEnabled {
		var rows []*ModelSupply
		if err := DB.Where("channel_id IN ? AND upstream_model IN ?", ids, names).Find(&rows).Error; err != nil {
			return nil, err
		}
		var models []string
		for _, s := range rows {
			supplies[s.ChannelId] = append(supplies[s.ChannelId], s)
			models = append(models, s.Model)
		}
		if len(models) > 0 {
			var infos []*ModelInfo
			if err := DB.Select("model", "status").Where("model IN ? AND status = ?", models, ModelStatusPublished).Find(&infos).Error; err != nil {
				return nil, err
			}
			for _, info := range infos {
				published[info.Model] = true
			}
		}
	}

	var resolver *CanonicalResolver
	out := make(map[int]*ChannelDriftImpact, len(flagged))
	for _, c := range channels {
		missing := flagged[c.Id]
		if len(missing) == 0 {
			continue
		}
		mapping := channelMappingOf(c)
		impact := &ChannelDriftImpact{Serving: []string{}}
		// The public model each missing name serves (or would serve), for the rename match.
		publicOf := map[string]string{}
		for _, m := range missing {
			upstream := upstreamNameOf(m, mapping)
			if !config.ModelSupplyEnabled {
				impact.Serving = append(impact.Serving, m)
				continue
			}
			serving := false
			for _, s := range supplies[c.Id] {
				if s.UpstreamModel != upstream && s.UpstreamModel != m {
					continue
				}
				publicOf[m] = s.Model
				if s.Enabled && !s.Pending && published[s.Model] {
					serving = true
				}
			}
			if serving {
				impact.Serving = append(impact.Serving, m)
			}
		}
		if drift := c.GetModelDrift(); drift != nil && len(drift.UpstreamSnapshot) > 0 {
			if resolver == nil {
				r, err := NewCanonicalResolver()
				if err != nil {
					return nil, err
				}
				resolver = r
			}
			impact.Renamed = upstreamRenames(resolver, missing, mapping, publicOf, drift.UpstreamSnapshot)
		}
		out[c.Id] = impact
	}
	return out, nil
}

// upstreamRenames pairs missing names with a name the upstream lists now for the same public
// model. Among several candidates the one whose bare id is the old name wins ("openai/gpt-5.5"
// for "gpt-5.5"), then the first the upstream lists.
func upstreamRenames(r *CanonicalResolver, missing []string, mapping, publicOf map[string]string, snapshot []string) map[string]string {
	publicOfName := func(name string) string {
		if info := r.publicModel(r.Resolve(name).Canonical); info != nil {
			return info.Model
		}
		return ""
	}
	byPublic := map[string][]string{}
	for _, name := range snapshot {
		if p := publicOfName(name); p != "" {
			byPublic[p] = append(byPublic[p], name)
		}
	}
	renamed := map[string]string{}
	for _, m := range missing {
		target := publicOf[m]
		if target == "" {
			target = publicOfName(m)
		}
		upstream := upstreamNameOf(m, mapping)
		var pick string
		for _, cand := range byPublic[target] {
			if cand == upstream {
				continue
			}
			if bareModelID(cand) == bareModelID(upstream) {
				pick = cand
				break
			}
			if pick == "" {
				pick = cand
			}
		}
		if target != "" && pick != "" {
			renamed[m] = pick
		}
	}
	if len(renamed) == 0 {
		return nil
	}
	return renamed
}

// AttachDriftImpacts fills DriftImpact on the channels before they are returned.
func AttachDriftImpacts(channels []*Channel) error {
	impacts, err := DriftImpacts(channels)
	if err != nil {
		return err
	}
	for _, c := range channels {
		c.DriftImpact = impacts[c.Id]
	}
	return nil
}

// ServedThroughChannel keeps the public models of names that are published and that the
// channel supplies with an enabled, confirmed supply (gone upstream or not): the ones whose
// supply changing state matters to users.
func ServedThroughChannel(channelID int, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var supplies []*ModelSupply
	if err := DB.Select("model").Where("channel_id = ? AND model IN ? AND enabled = ? AND pending = ?", channelID, names, true, false).Find(&supplies).Error; err != nil {
		return nil, err
	}
	var supplied []string
	for _, s := range supplies {
		supplied = append(supplied, s.Model)
	}
	if len(supplied) == 0 {
		return nil, nil
	}
	var infos []*ModelInfo
	if err := DB.Select("model").Where("model IN ? AND status = ?", supplied, ModelStatusPublished).Find(&infos).Error; err != nil {
		return nil, err
	}
	served := map[string]bool{}
	for _, info := range infos {
		served[info.Model] = true
	}
	var out []string
	for _, n := range names {
		if served[n] {
			out = append(out, n)
		}
	}
	return out, nil
}

// AdoptUpstreamRenames switches the channel from missing model names to the names the upstream
// renamed them to (see ChannelDriftImpact.Renamed): only renames the channel's current impact
// reports, limited to `only` when it is not empty. Returns how many were switched.
//
// While supplies route requests, a renamed model's supplies send the new name (and count as
// listed upstream again) and the channel's list takes the new name in place of the old one.
// Otherwise the listed name is what users call, so it stays and is mapped to the new name.
func AdoptUpstreamRenames(channel *Channel, only []string) (int, error) {
	impacts, err := DriftImpacts([]*Channel{channel})
	if err != nil {
		return 0, err
	}
	impact := impacts[channel.Id]
	if impact == nil || len(impact.Renamed) == 0 {
		return 0, nil
	}
	renames := map[string]string{}
	for from, to := range impact.Renamed {
		if len(only) == 0 || slices.Contains(only, from) {
			renames[from] = to
		}
	}
	if len(renames) == 0 {
		return 0, nil
	}

	mapping := channelMappingOf(channel)
	rawMapping := map[string]string{}
	if raw := strings.TrimSpace(channel.GetModelMapping()); raw != "" {
		_ = json.Unmarshal([]byte(raw), &rawMapping)
	}
	disabled := channel.DisabledModelSet()
	listed := SplitChannelModels(channel.Models)
	isListed := map[string]bool{}
	for _, m := range listed {
		isListed[m] = true
	}
	var models []string
	drop := map[string]bool{}
	for from, to := range renames {
		if config.ModelSupplyEnabled {
			drop[from] = true
			delete(rawMapping, from)
			delete(disabled, to)
			if !isListed[to] {
				isListed[to] = true
				listed = append(listed, to)
			}
			continue
		}
		prefix := ""
		if strings.HasPrefix(strings.TrimSpace(rawMapping[from]), "+") {
			prefix = "+"
		}
		rawMapping[from] = prefix + to
		// The new name joined the list on its own, switched off: the old name serves it now.
		if disabled[to] {
			drop[to] = true
			delete(disabled, to)
		}
	}
	for _, m := range listed {
		if !drop[m] {
			models = append(models, m)
		}
	}
	keptDisabled := datatypes.JSONSlice[string]{}
	for _, m := range models {
		if disabled[m] {
			keptDisabled = append(keptDisabled, m)
		}
	}
	mappingJSON := ""
	if len(rawMapping) > 0 {
		b, err := json.Marshal(rawMapping)
		if err != nil {
			return 0, err
		}
		mappingJSON = string(b)
	}

	drift := channel.GetModelDrift()
	if drift != nil {
		renamedTo := map[string]bool{}
		for _, to := range renames {
			renamedTo[to] = true
		}
		without := func(names []string, gone func(string) bool) []string {
			var out []string
			for _, n := range names {
				if !gone(n) {
					out = append(out, n)
				}
			}
			return out
		}
		isOld := func(n string) bool { return renames[n] != "" }
		drift.MissingModels = without(drift.MissingModels, isOld)
		drift.DismissedMissing = without(drift.DismissedMissing, isOld)
		drift.NewModels = without(drift.NewModels, func(n string) bool { return renamedTo[n] })
		drift.OK = len(drift.MissingModels) == 0
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		if config.ModelSupplyEnabled {
			for from, to := range renames {
				olds := []string{from, upstreamNameOf(from, mapping)}
				if err := tx.Model(&ModelSupply{}).Where("channel_id = ? AND upstream_model IN ?", channel.Id, olds).
					Updates(map[string]any{"upstream_model": to, "upstream_missing": 0, "upstream_gone_at": 0}).Error; err != nil {
					return err
				}
			}
		}
		updates := map[string]any{
			"models":          strings.Join(models, ","),
			"disabled_models": keptDisabled,
			"model_mapping":   mappingJSON,
		}
		if drift != nil {
			updates["model_drift"] = datatypes.NewJSONType(*drift)
		}
		return tx.Model(&Channel{}).Where("id = ?", channel.Id).Updates(updates).Error
	})
	if err != nil {
		return 0, err
	}
	channel.Models = strings.Join(models, ",")
	channel.DisabledModels = &keptDisabled
	channel.ModelMapping = &mappingJSON
	if drift != nil {
		d := datatypes.NewJSONType(*drift)
		channel.ModelDrift = &d
	}
	ChannelGroup.Load()
	return len(renames), nil
}

// ServedModelNames is the set of model names users are served now. While supplies route
// requests: published public models with an enabled, confirmed supply that its upstream still
// lists, on an enabled channel. Otherwise: names switched on in an enabled channel's list.
func ServedModelNames() (map[string]bool, error) {
	served := map[string]bool{}
	channels, err := GetAllChannels()
	if err != nil {
		return nil, err
	}
	enabled := map[int]bool{}
	for _, ch := range channels {
		if ch.Status != config.ChannelStatusEnabled {
			continue
		}
		enabled[ch.Id] = true
		if config.ModelSupplyEnabled {
			continue
		}
		disabled := ch.DisabledModelSet()
		for _, name := range SplitChannelModels(ch.Models) {
			if !disabled[name] {
				served[name] = true
			}
		}
	}
	if !config.ModelSupplyEnabled {
		return served, nil
	}
	var supplies []*ModelSupply
	if err := DB.Select("model", "channel_id").Where("enabled = ? AND pending = ? AND upstream_gone_at = ?", true, false, 0).Find(&supplies).Error; err != nil {
		return nil, err
	}
	supplied := map[string]bool{}
	for _, s := range supplies {
		if enabled[s.ChannelId] {
			supplied[s.Model] = true
		}
	}
	var infos []*ModelInfo
	if err := DB.Select("model").Where("status = ?", ModelStatusPublished).Find(&infos).Error; err != nil {
		return nil, err
	}
	for _, info := range infos {
		if supplied[info.Model] {
			served[info.Model] = true
		}
	}
	return served, nil
}
