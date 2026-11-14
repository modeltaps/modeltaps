package model

import (
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
)

// OrganizationAuditLog 组织关键操作审计日志(成员变更、角色变更、解散、令牌管理等)
type OrganizationAuditLog struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"type:int;not null;index:idx_org_audit_org_time,priority:1"`
	ActorId        int    `json:"actor_id" gorm:"type:int;not null;index"`
	Action         string `json:"action" gorm:"type:varchar(64);not null"`
	Content        string `json:"content" gorm:"type:varchar(1024);default:''"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint;index:idx_org_audit_org_time,priority:2"`
	// ActorName is the actor's username, filled in when listing (not stored).
	ActorName string `json:"actor_name" gorm:"-"`
}

// RecordOrgAudit 记录组织审计日志;失败仅写系统日志,不影响主流程
func RecordOrgAudit(organizationId int, actorId int, action string, content string) {
	log := &OrganizationAuditLog{
		OrganizationId: organizationId,
		ActorId:        actorId,
		Action:         action,
		Content:        content,
		CreatedTime:    utils.GetTimestamp(),
	}
	if err := DB.Create(log).Error; err != nil {
		logger.SysError("failed to record organization audit log: " + err.Error())
	}
}

// OrgAuditLogsParams 组织审计日志查询参数:按操作者/动作/时间过滤
type OrgAuditLogsParams struct {
	PaginationParams
	ActorId        int    `form:"actor_id"`
	Action         string `form:"action"`
	StartTimestamp int64  `form:"start_timestamp"`
	EndTimestamp   int64  `form:"end_timestamp"`
}

// GetOrganizationAuditLogs 按时间倒序分页查询组织审计日志(支持操作者/动作/时间过滤)
func GetOrganizationAuditLogs(organizationId int, params *OrgAuditLogsParams) (*DataResult[OrganizationAuditLog], error) {
	var logs []*OrganizationAuditLog
	db := DB.Where("organization_id = ?", organizationId)
	if params.ActorId != 0 {
		db = db.Where("actor_id = ?", params.ActorId)
	}
	if params.Action != "" {
		db = db.Where("action = ?", params.Action)
	}
	if params.StartTimestamp != 0 {
		db = db.Where("created_time >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp != 0 {
		db = db.Where("created_time <= ?", params.EndTimestamp)
	}
	result, err := PaginateAndOrder[OrganizationAuditLog](db, &params.PaginationParams, &logs, allowedOrgAuditOrderFields)
	if err != nil || result == nil || result.Data == nil {
		return result, err
	}
	fillAuditActorNames(*result.Data)
	return result, nil
}

// fillAuditActorNames sets ActorName from the users table; a failed lookup leaves names empty
// and the list still shows the actor ID.
func fillAuditActorNames(logs []*OrganizationAuditLog) {
	seen := make(map[int]bool)
	ids := make([]int, 0, len(logs))
	for _, log := range logs {
		if log.ActorId > 0 && !seen[log.ActorId] {
			seen[log.ActorId] = true
			ids = append(ids, log.ActorId)
		}
	}
	names, err := GetUsernamesByIds(ids)
	if err != nil {
		return
	}
	for _, log := range logs {
		log.ActorName = names[log.ActorId]
	}
}

var allowedOrgAuditOrderFields = map[string]bool{
	"id":           true,
	"created_time": true,
}
