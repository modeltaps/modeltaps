package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	return result, nil
}

// DriftedChannel 记录一个检出漂移或有上游新增模型的渠道，供通知与手动触发 API 汇报。
type DriftedChannel struct {
	ChannelId     int      `json:"channel_id"`
	ChannelName   string   `json:"channel_name"`
	MissingModels []string `json:"missing_models"`
	NewModels     []string `json:"new_models,omitempty"`
}

// ModelDriftCheckSummary 汇总一次全量漂移检测：写回结果的渠道数、检出漂移的渠道数、有上游新增的渠道数与其明细。
type ModelDriftCheckSummary struct {
	Checked  int              `json:"checked"`
	Drifted  int              `json:"drifted"`
	WithNew  int              `json:"with_new"`
	Channels []DriftedChannel `json:"channels"`
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
		if err := ch.UpdateModelDrift(*result); err != nil {
			logger.SysError("model drift persist failed for channel " + ch.Name + ": " + err.Error())
			continue
		}
		summary.Checked++
		if !result.OK {
			summary.Drifted++
		}
		if len(result.NewModels) > 0 {
			summary.WithNew++
		}
		if !result.OK || len(result.NewModels) > 0 {
			summary.Channels = append(summary.Channels, DriftedChannel{
				ChannelId:     ch.Id,
				ChannelName:   ch.Name,
				MissingModels: result.MissingModels,
				NewModels:     result.NewModels,
			})
		}
	}
	return summary, nil
}

// CheckAndNotifyModelDrift 供 cron 调用：执行全量漂移检测，检出缺失模型或上游新增模型时发通知。
// 通知内容含渠道名/ID 与缺失模型 / 上游新增模型清单（参照 DisableChannel 的通知用法）。
func CheckAndNotifyModelDrift() (*ModelDriftCheckSummary, error) {
	summary, err := CheckAllChannelsModelDrift()
	if err != nil {
		return nil, err
	}
	if summary.Drifted > 0 || summary.WithNew > 0 {
		notifyModelDrift(summary)
	}
	return summary, nil
}

// notifyModelDrift 组装漂移通知：标题含漂移 / 新增渠道数，正文逐条列出渠道名/ID 与缺失模型、上游新增模型。
func notifyModelDrift(summary *ModelDriftCheckSummary) {
	var parts []string
	if summary.Drifted > 0 {
		parts = append(parts, fmt.Sprintf("%d channel(s) with model drift", summary.Drifted))
	}
	if summary.WithNew > 0 {
		parts = append(parts, fmt.Sprintf("%d channel(s) with new upstream models", summary.WithNew))
	}
	subject := "Detected " + strings.Join(parts, ", ")

	var b strings.Builder
	for _, ch := range summary.Channels {
		if len(ch.MissingModels) > 0 {
			b.WriteString(fmt.Sprintf("Channel \"%s\" (#%d) is missing models: %s\n", ch.ChannelName, ch.ChannelId, strings.Join(ch.MissingModels, ", ")))
		}
		if len(ch.NewModels) > 0 {
			b.WriteString(fmt.Sprintf("Channel \"%s\" (#%d) has new upstream models: %s\n", ch.ChannelName, ch.ChannelId, strings.Join(ch.NewModels, ", ")))
		}
	}
	notify.Send(subject, b.String())
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
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		result, err := CheckChannelModelDrift(channel)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
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
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid channel ID"})
		return
	}

	channel, err := model.GetChannelById(req.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := channel.DismissNewModels(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channel.GetModelDrift()})
}

// fetchUpstreamModels 构造 provider 并拉取上游实时模型列表（取 key 首行，同 GetModelList）。
func fetchUpstreamModels(channel *model.Channel) ([]string, error) {
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
		OK:               len(missing) == 0,
	}
}

// computeNewModels 计算累计的「上游新增且渠道未配置」模型列表，顺序为 prev.NewModels 原序 + 本次新增按上游顺序。
func computeNewModels(snapshot []string, upstreamSet, configured map[string]bool, prev *model.ModelDriftResult) []string {
	var newModels []string
	seen := make(map[string]bool)
	appendIfEligible := func(name string) {
		if name == "" || seen[name] || configured[name] || !upstreamSet[name] {
			return
		}
		seen[name] = true
		newModels = append(newModels, name)
	}

	if prev != nil {
		for _, m := range prev.NewModels {
			appendIfEligible(strings.TrimSpace(m))
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
			appendIfEligible(u)
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
