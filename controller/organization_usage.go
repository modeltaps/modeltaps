package controller

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// resolveOrgUsageMemberFilter 应用成员用量可见性策略(规格 §3.6 / a4):
// Owner/Admin 或组织开启互见时按请求过滤;Member 且关闭互见时只能看自己,指定他人时报错。
func resolveOrgUsageMemberFilter(c *gin.Context, org *model.Organization, role string, requestedMemberId int) (int, error) {
	if role == model.OrgRoleOwner || role == model.OrgRoleAdmin || org.IsUsageVisibleToMembers() {
		return requestedMemberId, nil
	}
	selfId := c.GetInt("id")
	if requestedMemberId != 0 && requestedMemberId != selfId {
		return 0, errors.New("The current organization settings do not allow viewing other members' usage")
	}
	return selfId, nil
}

// orgMemberIdFromLog 从日志 metadata 读取实际成员 ID(JSON 反序列化后数字为 float64)
func orgMemberIdFromLog(log *model.Log) int {
	meta := log.Metadata.Data()
	if meta == nil {
		return 0
	}
	switch v := meta["org_member_id"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

// attachOrgMemberUsernames 把日志的展示用户名从影子账户改写为实际成员用户名
func attachOrgMemberUsernames(logs []*model.Log) {
	idSet := make(map[int]bool)
	for _, log := range logs {
		if id := orgMemberIdFromLog(log); id > 0 {
			idSet[id] = true
		}
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	usernames, err := model.GetUsernamesByIds(ids)
	if err != nil {
		return
	}
	for _, log := range logs {
		if id := orgMemberIdFromLog(log); id > 0 {
			if name, ok := usernames[id]; ok {
				log.Username = name
			}
		}
	}
}

// GetOrgLogs 组织日志分页列表(Member+,受可见性策略约束)
func GetOrgLogs(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.OrgLogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	memberId, err := resolveOrgUsageMemberFilter(c, org, role, params.MemberId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.MemberId = memberId
	result, err := model.GetOrgLogsList(org.ShadowUserId, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	attachOrgMemberUsernames(*result.Data)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

// GetOrgLogsHistogram 组织上下文的请求量直方图(Member+,受可见性策略约束,桶粒度按窗口自适应)。
func GetOrgLogsHistogram(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.OrgLogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if checkLogTimeRange(c, params.StartTimestamp, params.EndTimestamp) {
		return
	}
	memberId, err := resolveOrgUsageMemberFilter(c, org, role, params.MemberId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.MemberId = memberId
	bucket := histogramBucketType(params.StartTimestamp, params.EndTimestamp)
	buckets, err := model.GetOrgLogsRequestHistogram(org.ShadowUserId, &params, bucket)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"bucket":  bucket,
			"buckets": buckets,
		},
	})
}

// GetOrgLogDetail 组织上下文返回单条日志的完整请求/响应明细(T50f)。
// 以 CanViewTokenLogIO 为唯一鉴权闸门(组织角色语义),并要求日志/明细归属本组织影子账户。
func GetOrgLogDetail(c *gin.Context) {
	org, role := getOrgFromContext(c)
	id, err := strconv.Atoi(c.Param("log_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondLogDetail(c, id, c.GetInt("id"), c.GetInt("role"), role, org.ShadowUserId)
}

// GetOrgLogsStat 组织消费总额统计(Member+,受可见性策略约束)
func GetOrgLogsStat(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.OrgLogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	memberId, err := resolveOrgUsageMemberFilter(c, org, role, params.MemberId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.MemberId = memberId
	quota := model.SumOrgUsedQuota(org.ShadowUserId, &params)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quota,
		},
	})
}

// GetOrgUsageAnalytics 组织用量聚合(按成员/令牌/模型/时间维度,Member+ 受可见性策略约束)
func GetOrgUsageAnalytics(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.OrgLogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	groupBy := c.DefaultQuery("group_by", model.OrgUsageGroupMember)
	memberId, err := resolveOrgUsageMemberFilter(c, org, role, params.MemberId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.MemberId = memberId
	statistics, err := model.GetOrgUsageStatistics(org.ShadowUserId, groupBy, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if groupBy == model.OrgUsageGroupMember {
		attachOrgUsageUsernames(statistics)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    statistics,
	})
}

// attachOrgUsageUsernames 为按成员聚合的结果补充用户名
func attachOrgUsageUsernames(statistics []*model.OrgUsageStatistic) {
	ids := make([]int, 0, len(statistics))
	for _, stat := range statistics {
		if stat.MemberId > 0 {
			ids = append(ids, stat.MemberId)
		}
	}
	usernames, err := model.GetUsernamesByIds(ids)
	if err != nil {
		return
	}
	for _, stat := range statistics {
		if name, ok := usernames[stat.MemberId]; ok {
			stat.Username = name
		}
	}
}

// ExportOrgLogs 组织日志 CSV 导出(Member+,受可见性策略约束;规格 §3.6 / b9)
func ExportOrgLogs(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.OrgLogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if checkLogTimeRange(c, params.StartTimestamp, params.EndTimestamp) {
		return
	}
	memberId, err := resolveOrgUsageMemberFilter(c, org, role, params.MemberId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.MemberId = memberId
	logs, err := model.GetAllOrgLogsList(org.ShadowUserId, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	attachOrgMemberUsernames(logs)

	filename := fmt.Sprintf("org_logs_export_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	headers := []string{
		"Time", "Member", "API Key", "Type", "Model",
		"Duration (s)", "Input tokens", "Output tokens", "Quota", "Details",
	}
	if err := writer.Write(headers); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	for _, log := range logs {
		if err := writer.Write(formatOrgLogToCSVRow(log)); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
}

// ExportOrgUsageAnalytics 组织用量聚合 CSV 导出(Member+,受可见性策略约束;W8-H)。
// 鉴权/过滤/可见性策略与 GetOrgUsageAnalytics 完全一致(同路由组 + resolveOrgUsageMemberFilter);
// 一次导出按日趋势与成员/令牌/模型三维度,复用 ExportOrgLogs 的 csv.NewWriter 转义管线。
func ExportOrgUsageAnalytics(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.OrgLogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if checkLogTimeRange(c, params.StartTimestamp, params.EndTimestamp) {
		return
	}
	memberId, err := resolveOrgUsageMemberFilter(c, org, role, params.MemberId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.MemberId = memberId

	byDate, err := model.GetOrgUsageStatistics(org.ShadowUserId, model.OrgUsageGroupDate, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	byMember, err := model.GetOrgUsageStatistics(org.ShadowUserId, model.OrgUsageGroupMember, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	attachOrgUsageUsernames(byMember)
	byToken, err := model.GetOrgUsageStatistics(org.ShadowUserId, model.OrgUsageGroupToken, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	byModel, err := model.GetOrgUsageStatistics(org.ShadowUserId, model.OrgUsageGroupModel, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	filename := fmt.Sprintf("org_usage_export_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	headers := []string{
		"Dimension", "Name", "Requests", "Input tokens", "Output tokens",
		"Cache read tokens", "Quota (USD)", "Saved quota (USD)",
	}
	if err := writer.Write(headers); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	writeDimension := func(dimLabel string, stats []*model.OrgUsageStatistic, nameOf func(*model.OrgUsageStatistic) string) error {
		for _, stat := range stats {
			if err := writer.Write(formatOrgUsageStatToCSVRow(dimLabel, nameOf(stat), stat)); err != nil {
				return err
			}
		}
		return nil
	}

	memberName := func(stat *model.OrgUsageStatistic) string {
		if stat.Username != "" {
			return stat.Username
		}
		if stat.MemberId > 0 {
			return fmt.Sprintf("#%d", stat.MemberId)
		}
		return ""
	}

	if err := writeDimension("Date", byDate, func(s *model.OrgUsageStatistic) string { return s.Date }); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := writeDimension("Member", byMember, memberName); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := writeDimension("API Key", byToken, func(s *model.OrgUsageStatistic) string { return s.TokenName }); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := writeDimension("Model", byModel, func(s *model.OrgUsageStatistic) string { return s.ModelName }); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
}

// formatOrgUsageStatToCSVRow 组织用量聚合导出行;额度/节省额换算为 USD,与页面 calculateQuota 口径一致。
func formatOrgUsageStatToCSVRow(dimLabel, name string, stat *model.OrgUsageStatistic) []string {
	if name == "" {
		name = "-"
	}
	return []string{
		dimLabel,
		csvSafeField(name),
		strconv.FormatInt(stat.RequestCount, 10),
		strconv.FormatInt(stat.PromptTokens, 10),
		strconv.FormatInt(stat.CompletionTokens, 10),
		strconv.FormatInt(stat.CachedReadTokens, 10),
		fmt.Sprintf("%.6f", float64(stat.Quota)/config.QuotaPerUnit),
		fmt.Sprintf("%.6f", stat.SavedQuota/config.QuotaPerUnit),
	}
}

// formatOrgLogToCSVRow 组织日志导出行(不含渠道/IP 等站点内部信息)
func formatOrgLogToCSVRow(log *model.Log) []string {
	durationStr := ""
	if log.RequestTime > 0 {
		durationStr = fmt.Sprintf("%.2f", float64(log.RequestTime)/1000.0)
	}
	quotaStr := "0"
	if log.Quota > 0 {
		quotaStr = fmt.Sprintf("%.6f", float64(log.Quota)/config.QuotaPerUnit)
	}
	inputTokensStr := ""
	if log.PromptTokens > 0 {
		inputTokensStr = strconv.Itoa(log.PromptTokens)
	}
	outputTokensStr := ""
	if log.CompletionTokens > 0 {
		outputTokensStr = strconv.Itoa(log.CompletionTokens)
	}
	return []string{
		time.Unix(log.CreatedAt, 0).Format("2006-01-02 15:04:05"),
		csvSafeField(log.Username),
		csvSafeField(log.TokenName),
		getLogTypeText(log.Type),
		csvSafeField(log.ModelName),
		durationStr,
		inputTokensStr,
		outputTokensStr,
		quotaStr,
		csvSafeField(log.Content),
	}
}
