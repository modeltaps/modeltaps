package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"net/http"
	"slices"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"

	"github.com/gin-gonic/gin"
)

// ErrModelListNotSupported 表示该渠道对应的 provider 未实现 ModelListInterface，
// 无法向上游拉取实时模型列表，漂移检测应跳过（非漂移、非错误）。
var ErrModelListNotSupported = errors.New("channel provider does not support model list")

// CheckChannelModelDrift 对单个渠道执行「渠道 models vs 上游实时模型列表」对比。
// 复用 GetProvider + ModelListInterface.GetModelList()（nil context 下仅做无鉴权 GET）。
// 上游拉取失败或不支持列表能力时返回错误，由调用方决定跳过——绝不写入误报结果。
func CheckChannelModelDrift(channel *model.Channel) (*model.ModelDriftResult, error) {
	upstream, err := fetchUpstreamModels(channel)
	result, ok := evaluateDrift(channel.Models, channel.GetModelMapping(), upstream, err, channel.GetModelDrift())
	if !ok {
		return nil, err
	}
	result.NewModels = filterIgnoredModels(result.NewModels, newDriftIgnoreMatcher(channel.DriftIgnoreRules))
	// A model a supply on this channel already sends upstream is configured, whatever the
	// channel's own model list says.
	result.NewModels = model.WithoutSuppliedModels(channel.Id, result.NewModels)
	if config.ModelSupplyEnabled {
		change, err := model.MarkSupplyUpstreams(channel, upstream)
		if err != nil {
			logger.SysError("supply upstream check failed for channel " + channel.Name + ": " + err.Error())
		} else if !change.Empty() || len(change.Missing) > 0 {
			result.Supplies = change
		}
	}
	return result, nil
}

func filterIgnoredModels(models []string, matcher *driftIgnoreMatcher) []string {
	var kept []string
	for _, m := range models {
		if !matcher.ignored(m) {
			kept = append(kept, m)
		}
	}
	return kept
}

// syncDriftModels keeps a channel's model list in step with its upstream and returns the new
// upstream models that route at once. Every name the upstream lists that the channel does not
// list (by name, as a mapping target or under an enabled wildcard), did not have removed and does
// not ignore joins the list. While supplies route requests the list serves nothing by itself, so
// names join switched on and the supplies decide; otherwise a name joins switched off unless it
// is new at this check and the channel adds new models automatically.
//
// With auto add on, the new names are then handled as before: while supplies route requests the
// channel's intake rules decide, for each new name that belongs to a public model, whether a
// supply is added, switched on or left for an admin to confirm (see model.IntakeChannelModels);
// otherwise the names switched on get their prices and metadata. Names handled this way are
// taken off the result's new models; the rest stay flagged for an admin.
func syncDriftModels(ch *model.Channel, result *model.ModelDriftResult) []string {
	matcher := newDriftIgnoreMatcher(ch.DriftIgnoreRules)
	listed := ch.ListedUpstreamNames()
	removed := make(map[string]bool, len(result.Removed))
	for _, name := range result.Removed {
		removed[name] = true
	}
	fresh := make(map[string]bool, len(result.NewModels))
	for _, name := range result.NewModels {
		fresh[name] = true
	}
	var join, off, switchedOn []string
	for _, name := range result.UpstreamSnapshot {
		if name == "" || listed[name] || removed[name] || strings.Contains(name, "*") || matcher.ignored(name) || ch.CoveredByWildcard(name) {
			continue
		}
		join = append(join, name)
		switch {
		case config.ModelSupplyEnabled:
		case ch.DriftAutoAdd && fresh[name]:
			switchedOn = append(switchedOn, name)
		default:
			off = append(off, name)
		}
	}
	if err := ch.JoinUpstreamModels(join, off); err != nil {
		logger.SysError("model drift sync failed for channel " + ch.Name + ": " + err.Error())
		return nil
	}
	if !ch.DriftAutoAdd || len(result.NewModels) == 0 {
		return nil
	}
	if config.ModelSupplyEnabled {
		outcomes, err := model.IntakeChannelModels(ch, result.NewModels)
		if err != nil {
			logger.SysError("model drift intake failed for channel " + ch.Name + ": " + err.Error())
			return nil
		}
		result.Intake = outcomes
		var added []string
		for _, o := range outcomes {
			if o.Action == model.IntakeActionEnable {
				added = append(added, o.Name)
			}
		}
		result.NewModels = model.WithoutSuppliedModels(ch.Id, result.NewModels)
		return added
	}
	if len(switchedOn) == 0 {
		return nil
	}
	result.NewModels = slices.DeleteFunc(result.NewModels, func(name string) bool { return slices.Contains(switchedOn, name) })
	if len(result.NewModels) == 0 {
		result.NewModels = nil
	}
	AutoSyncChannelCatalog(ch.Type, strings.Join(switchedOn, ","), "drift auto add")
	return switchedOn
}

// DriftedChannel 记录一个检出漂移或有上游新增模型的渠道，供通知与手动触发 API 汇报。
type DriftedChannel struct {
	ChannelId     int      `json:"channel_id"`
	ChannelName   string   `json:"channel_name"`
	MissingModels []string `json:"missing_models"`
	// Renamed maps missing models to the name the upstream now lists for the same public model.
	Renamed     map[string]string `json:"renamed,omitempty"`
	NewModels   []string          `json:"new_models,omitempty"`
	AddedModels []string          `json:"added_models,omitempty"`
	// PendingModels are new upstream models the intake rules added for an admin to confirm.
	PendingModels []string `json:"pending_models,omitempty"`
	// Supplies is what the check changed on the channel's supplies (gone / back / held).
	Supplies *model.SupplyUpstreamChange `json:"supplies,omitempty"`
}

// ModelDriftCheckSummary 汇总一次全量漂移检测：写回结果的渠道数、检出漂移的渠道数、有上游新增的渠道数与其明细。
type ModelDriftCheckSummary struct {
	Checked int `json:"checked"`
	Drifted int `json:"drifted"`
	WithNew int `json:"with_new"`
	Added   int `json:"added"`
	// SuppliesChanged counts channels with supplies marked gone or back, or held.
	SuppliesChanged int              `json:"supplies_changed"`
	Channels        []DriftedChannel `json:"channels"`
	// checkedIds are the channels whose check succeeded, for closing the alerts of the ones
	// with nothing to report.
	checkedIds []int
}

// CheckAllChannelsModelDrift 遍历所有启用渠道，仅对实现 ModelListInterface 的 provider 检测。
// 单渠道失败（上游拉取失败 / 不支持）不中断全局：记日志并跳过，不写入误报结果。
// 返回检测汇总（写回渠道数、漂移渠道数及其明细）。
func CheckAllChannelsModelDrift() (*ModelDriftCheckSummary, error) {
	channels, err := model.GetAllChannels()
	if err != nil {
		return nil, err
	}

	summary := &ModelDriftCheckSummary{}
	for _, ch := range channels {
		if ch.Status != config.ChannelStatusEnabled {
			continue
		}
		result, err := CheckChannelModelDrift(ch)
		if err != nil {
			if !errors.Is(err, ErrModelListNotSupported) {
				logger.SysError("model drift check failed for channel " + ch.Name + ": " + err.Error())
			}
			continue
		}
		added := syncDriftModels(ch, result)
		if err := ch.UpdateModelDrift(*result); err != nil {
			logger.SysError("model drift persist failed for channel " + ch.Name + ": " + err.Error())
			continue
		}
		summary.Checked++
		summary.checkedIds = append(summary.checkedIds, ch.Id)
		if len(added) > 0 {
			summary.Added++
		}
		// Missing models an admin dismissed or switched off are not reported again, nor the ones
		// no public model is served through: their going away changes nothing for users.
		missing, renamed := servingMissing(ch, result)
		if len(missing) > 0 {
			summary.Drifted++
		}
		if len(result.NewModels) > 0 {
			summary.WithNew++
		}
		pending := pendingIntake(result.Intake)
		var supplies *model.SupplyUpstreamChange
		if !result.Supplies.Empty() {
			supplies = result.Supplies
			summary.SuppliesChanged++
		}
		if len(missing) > 0 || len(result.NewModels) > 0 || len(added) > 0 || len(pending) > 0 || supplies != nil {
			summary.Channels = append(summary.Channels, DriftedChannel{
				ChannelId:     ch.Id,
				ChannelName:   ch.Name,
				MissingModels: missing,
				Renamed:       renamed,
				NewModels:     result.NewModels,
				AddedModels:   added,
				PendingModels: pending,
				Supplies:      supplies,
			})
		}
	}
	return summary, nil
}

// servingMissing is the missing models of a check worth telling an admin about: not dismissed
// or switched off, and served to users (see model.DriftImpacts), with the upstream's new name
// for those it renamed. When the impact cannot be worked out every unhandled missing model counts.
func servingMissing(ch *model.Channel, result *model.ModelDriftResult) ([]string, map[string]string) {
	impacts, err := model.DriftImpacts([]*model.Channel{ch})
	if err != nil {
		logger.SysError("model drift impact failed for channel " + ch.Name + ": " + err.Error())
		return undismissedMissing(result, ch.DisabledModelSet()), nil
	}
	impact := impacts[ch.Id]
	if impact == nil || len(impact.Serving) == 0 {
		return nil, nil
	}
	renamed := map[string]string{}
	for _, m := range impact.Serving {
		if to := impact.Renamed[m]; to != "" {
			renamed[m] = to
		}
	}
	if len(renamed) == 0 {
		renamed = nil
	}
	return impact.Serving, renamed
}

// CheckAndNotifyModelDrift 供 cron 调用：执行全量漂移检测，只在会影响服务时发通知：在服务中的模型
// 上游不再列出（含改名）、线路因此停用、或上游列表异常。上游新增、自动加入和待确认的模型是资讯，
// 在渠道页查看，不发通知。
func CheckAndNotifyModelDrift() (*ModelDriftCheckSummary, error) {
	summary, err := CheckAllChannelsModelDrift()
	if err != nil {
		return nil, err
	}
	if subject, body := modelDriftNotice(summary, reportDriftAlert); body != "" {
		notify.SendKind(notify.KindModelDrift, subject, body)
	}
	return summary, nil
}

// reportDriftAlert records a checked channel's drift items as its alert and returns the new
// ones; when the alert cannot be read every item counts as new, so no notice is lost.
func reportDriftAlert(channelID int, items []string) []string {
	fresh, err := model.ReportAlert(notify.KindModelDrift, fmt.Sprintf("channel:%d", channelID), items)
	if err != nil {
		logger.SysError("model drift alert failed: " + err.Error())
		return items
	}
	return fresh
}

// driftFacts is what a checked channel's drift means for users, as lines for the notice and
// alert items (stable strings, so the same facts at the next check are not new).
type driftFacts struct {
	renamed, gone, stopped, unsupplied []string
	held                               bool
}

func driftFactsOf(ch DriftedChannel) driftFacts {
	var f driftFacts
	for _, m := range ch.MissingModels {
		if to := ch.Renamed[m]; to != "" {
			f.renamed = append(f.renamed, m+" → "+to)
		} else {
			f.gone = append(f.gone, m)
		}
	}
	if c := ch.Supplies; c != nil {
		// Only supplies that served users: a switched-off supply or one of an unpublished
		// model going away changes nothing.
		f.stopped, f.unsupplied = servedOnly(ch.ChannelId, c.Gone), servedOnly(ch.ChannelId, c.Unsupplied)
		f.held = c.Held
	}
	return f
}

func (f driftFacts) items() []string {
	var out []string
	for _, x := range f.renamed {
		out = append(out, "renamed:"+x)
	}
	for _, x := range f.gone {
		out = append(out, "gone:"+x)
	}
	for _, x := range f.stopped {
		out = append(out, "stopped:"+x)
	}
	for _, x := range f.unsupplied {
		out = append(out, "unsupplied:"+x)
	}
	if f.held {
		out = append(out, "held")
	}
	return out
}

func (f driftFacts) write(b *strings.Builder, name string, id int) {
	if len(f.renamed) > 0 {
		b.WriteString(fmt.Sprintf("Channel \"%s\" (#%d): the upstream renamed models still in use; switch to the new names from the channel's models tab: %s\n", name, id, strings.Join(f.renamed, ", ")))
	}
	if len(f.gone) > 0 {
		b.WriteString(fmt.Sprintf("Channel \"%s\" (#%d): the upstream no longer lists models still in use: %s\n", name, id, strings.Join(f.gone, ", ")))
	}
	if len(f.stopped) > 0 {
		b.WriteString(fmt.Sprintf("Channel \"%s\" (#%d) stopped supplying models its upstream no longer lists: %s\n", name, id, strings.Join(f.stopped, ", ")))
	}
	if len(f.unsupplied) > 0 {
		b.WriteString(fmt.Sprintf("No supply can serve these models any more: %s\n", strings.Join(f.unsupplied, ", ")))
	}
	if f.held {
		b.WriteString(fmt.Sprintf("Channel \"%s\" (#%d): the upstream listed none of the supplied models; nothing was marked gone, check its model list\n", name, id))
	}
}

// modelDriftNotice is the subject and body of the drift notice: the channels whose drift has
// something new for users since the alert last told it (report records each checked channel's
// items and returns the new ones). A channel whose drift goes on unchanged, shrinks or ends
// sends nothing; one with something new is told in full. Empty body: nothing to send.
func modelDriftNotice(summary *ModelDriftCheckSummary, report func(int, []string) []string) (string, string) {
	var b strings.Builder
	affected := 0
	drifted := map[int]bool{}
	for _, ch := range summary.Channels {
		drifted[ch.ChannelId] = true
		facts := driftFactsOf(ch)
		if len(report(ch.ChannelId, facts.items())) == 0 {
			continue
		}
		facts.write(&b, ch.ChannelName, ch.ChannelId)
		affected++
	}
	// Checked channels with nothing to say close their alert.
	for _, id := range summary.checkedIds {
		if !drifted[id] {
			report(id, nil)
		}
	}
	if affected == 0 {
		return "", ""
	}
	return fmt.Sprintf("Upstream model changes affect %d channel(s)", affected), b.String()
}

// CheckModelDrift 手动触发漂移检测（AdminAuth）。
// 请求体可选 {"id": <channelId>}：给定则只检测该渠道，否则全量。返回检测统计。
func CheckModelDrift(c *gin.Context) {
	var req struct {
		Id int `json:"id"`
	}
	_ = c.ShouldBindJSON(&req)

	if req.Id > 0 {
		channel, err := model.GetChannelById(req.Id)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
			return
		}
		result, err := CheckChannelModelDrift(channel)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		syncDriftModels(channel, result)
		if err := channel.UpdateModelDrift(*result); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
		return
	}

	summary, err := CheckAllChannelsModelDrift()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}

// DismissNewModels 忽略某渠道的上游新增模型标记（AdminAuth）。
// 请求体 {"id": <channelId>}：清空该渠道 NewModels，保留缺失模型与上游快照（不会再次报同一批模型）。
func DismissNewModels(c *gin.Context) {
	var req struct {
		Id int `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Id <= 0 {
		common.APIRespondWithError(c, http.StatusOK, invalidChannelIDError())
		return
	}

	channel, err := model.GetChannelById(req.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
		return
	}
	if err := channel.DismissNewModels(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channel.GetModelDrift()})
}

// DismissMissingModels stops flagging the models a channel's last drift check found missing
// (AdminAuth). Body: {"id": <channelId>}. Returns the updated drift result.
func DismissMissingModels(c *gin.Context) {
	var req struct {
		Id int `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Id <= 0 {
		common.APIRespondWithError(c, http.StatusOK, invalidChannelIDError())
		return
	}

	channel, err := model.GetChannelById(req.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
		return
	}
	if err := channel.DismissMissingModels(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channel.GetModelDrift()})
}

// AdoptUpstreamRenames switches a channel from models its upstream renamed to their new names
// (AdminAuth). Body: {"id": <channelId>, "models": [<old names>]}; no models switches every
// rename the channel's drift impact reports. Returns how many were switched and the channel.
func AdoptUpstreamRenames(c *gin.Context) {
	var req struct {
		Id     int      `json:"id"`
		Models []string `json:"models"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Id <= 0 {
		common.APIRespondWithError(c, http.StatusOK, invalidChannelIDError())
		return
	}
	channel, err := model.GetChannelById(req.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
		return
	}
	switched, err := model.AdoptUpstreamRenames(channel, req.Models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"switched": switched}})
}

// fetchUpstreamModels 构造 provider 并拉取上游实时模型列表（取 key 首行，同 GetModelList）。
// The drift check runs from a scheduled job with no recover above it, so a panic in one
// provider would stop the whole process: turn it into an error and the channel is skipped.
func fetchUpstreamModels(channel *model.Channel) (models []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			models = nil
			err = fmt.Errorf("model list panicked: %v", r)
		}
	}()
	// 拷贝一份用于取 key 首行，避免污染调用方持有的渠道对象。
	probe := *channel
	keys := strings.Split(probe.Key, "\n")
	probe.Key = keys[0]

	provider := providers.GetProvider(&probe, nil)
	if provider == nil {
		return nil, ErrModelListNotSupported
	}
	modelProvider, ok := provider.(providersBase.ModelListInterface)
	if !ok {
		return nil, ErrModelListNotSupported
	}
	return modelProvider.GetModelList()
}

// evaluateDrift 是漂移判定的纯函数：给定渠道 models、model_mapping、上游列表、拉取错误与上次结果，
// 返回结果与是否应写回。fetchErr != nil 时返回 (nil, false)，即跳过不写入——避免上游拉取失败被误报为漂移。
func evaluateDrift(modelsCSV, modelMapping string, upstream []string, fetchErr error, prev *model.ModelDriftResult) (*model.ModelDriftResult, bool) {
	if fetchErr != nil {
		return nil, false
	}
	result := computeModelDrift(modelsCSV, modelMapping, upstream, prev)
	return &result, true
}

// computeModelDrift 对比渠道配置的模型与上游实时列表。
// 渠道 models 中的名称若在 model_mapping 中有映射（对外别名），按映射后的上游名对比；
// 否则按原始名对比。MissingModels 保留渠道侧名称，便于管理员定位并修复。
//
// 「上游新增」以 prev.UpstreamSnapshot（上次检测的上游完整列表）为基线，而非渠道 models——
// 否则上游有数百模型而渠道只配十几个时会把未配置的模型全部报为新增。
// NewModels = (prev.NewModels ∪ (upstream − prevSnapshot)) − 渠道已配置模型，且只保留仍在上游的名称。
// prevSnapshot 为空（首次检测）时不追加新增，仅落快照建立基线。
// OK 语义保持「无缺失」——上游新增不算异常。
func computeModelDrift(modelsCSV, modelMapping string, upstream []string, prev *model.ModelDriftResult) model.ModelDriftResult {
	snapshot := make([]string, 0, len(upstream))
	upstreamSet := make(map[string]bool, len(upstream))
	for _, u := range upstream {
		u = strings.TrimSpace(u)
		if u != "" && !upstreamSet[u] {
			upstreamSet[u] = true
			snapshot = append(snapshot, u)
		}
	}

	modelMap := parseModelMapping(modelMapping)

	// configured 同时收录渠道侧名称与映射后的上游名，供缺失判定与新增排除共用。
	configured := make(map[string]bool)
	var missing []string
	for _, m := range strings.Split(modelsCSV, ",") {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		realName := m
		if mapped := modelMap[m]; mapped != "" {
			realName = mapped
		}
		configured[m] = true
		configured[realName] = true
		if !upstreamSet[realName] {
			missing = append(missing, m)
		}
	}

	return model.ModelDriftResult{
		CheckedAt:        utils.GetTimestamp(),
		MissingModels:    missing,
		NewModels:        computeNewModels(snapshot, upstreamSet, configured, prev),
		UpstreamSnapshot: snapshot,
		DismissedMissing: stillDismissed(missing, prev),
		Removed:          stillRemoved(configured, prev),
		OK:               len(missing) == 0,
	}
}

// stillDismissed keeps the dismissed missing models of the previous check that are still
// missing, so a dismissal lasts while the model stays missing and no longer.
func stillDismissed(missing []string, prev *model.ModelDriftResult) []string {
	if prev == nil || len(prev.DismissedMissing) == 0 {
		return nil
	}
	var kept []string
	for _, m := range prev.DismissedMissing {
		if slices.Contains(missing, m) {
			kept = append(kept, m)
		}
	}
	return kept
}

// stillRemoved keeps the removed names of the previous check that the channel does not list
// again, so a removal lasts until the admin adds the model back.
func stillRemoved(configured map[string]bool, prev *model.ModelDriftResult) []string {
	if prev == nil {
		return nil
	}
	var kept []string
	for _, m := range prev.Removed {
		if !configured[m] {
			kept = append(kept, m)
		}
	}
	return kept
}

// undismissedMissing is the missing models of a drift result an admin has not handled yet:
// neither dismissed nor switched off on the channel (disabled, keyed by the channel's names).
func undismissedMissing(result *model.ModelDriftResult, disabled map[string]bool) []string {
	var out []string
	for _, m := range result.MissingModels {
		if !slices.Contains(result.DismissedMissing, m) && !disabled[m] {
			out = append(out, m)
		}
	}
	return out
}

// computeNewModels 计算累计的「上游新增」模型列表，顺序为 prev.NewModels 原序 + 本次新增按上游顺序。
// 上次已标记的名称在检测把它加入渠道列表后仍保留标记（直到管理员忽略、开关或移除），只要上游仍列出且未被移除；
// 本次新出现的名称若渠道早已配置（管理员先手动加过）则不标记。
func computeNewModels(snapshot []string, upstreamSet, configured map[string]bool, prev *model.ModelDriftResult) []string {
	var newModels []string
	seen := make(map[string]bool)
	removed := make(map[string]bool)
	if prev != nil {
		for _, m := range prev.Removed {
			removed[m] = true
		}
	}
	appendIfEligible := func(name string, flagged bool) {
		if name == "" || seen[name] || removed[name] || !upstreamSet[name] || (!flagged && configured[name]) {
			return
		}
		seen[name] = true
		newModels = append(newModels, name)
	}

	if prev != nil {
		for _, m := range prev.NewModels {
			appendIfEligible(strings.TrimSpace(m), true)
		}
	}

	// 首次检测（无基线快照）只建立基线，不产生新增噪音。
	if prev == nil || len(prev.UpstreamSnapshot) == 0 {
		return newModels
	}

	prevSet := make(map[string]bool, len(prev.UpstreamSnapshot))
	for _, m := range prev.UpstreamSnapshot {
		if m = strings.TrimSpace(m); m != "" {
			prevSet[m] = true
		}
	}
	for _, u := range snapshot {
		if !prevSet[u] {
			appendIfEligible(u, false)
		}
	}
	return newModels
}

// parseModelMapping 解析渠道 model_mapping（对外别名 -> 上游真实名）。解析失败按无映射处理。
func parseModelMapping(modelMapping string) map[string]string {
	modelMap := make(map[string]string)
	if modelMapping == "" || modelMapping == "{}" {
		return modelMap
	}
	if err := json.Unmarshal([]byte(modelMapping), &modelMap); err != nil {
		return make(map[string]string)
	}
	return modelMap
}

// pendingIntake returns the upstream names the intake rules left waiting for an admin.
func pendingIntake(outcomes []model.IntakeOutcome) []string {
	var out []string
	for _, o := range outcomes {
		if o.Action == model.IntakeActionPending {
			out = append(out, o.Name)
		}
	}
	return out
}

// servedOnly keeps the public models the channel serves users through (see
// model.ServedThroughChannel); all of them when that cannot be read, so no notice is lost.
func servedOnly(channelID int, names []string) []string {
	out, err := model.ServedThroughChannel(channelID, names)
	if err != nil {
		logger.SysError("served models lookup failed: " + err.Error())
		return names
	}
	return out
}
