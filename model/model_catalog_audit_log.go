package model

import (
	"encoding/json"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
)

// 目录管理动作：站点级，不属于任何组织，故不复用 organization_audit_log。
const (
	ModelCatalogActionHide      = "catalog.hide"
	ModelCatalogActionUnhide    = "catalog.unhide"
	ModelCatalogActionApplySeed = "catalog.apply_seed"
)

// 旧的发布 / 下架动作：不再写入，只供 hidden 迁移回放历史审计。
const (
	ModelCatalogActionLegacyPublish   = "catalog.publish"
	ModelCatalogActionLegacyUnpublish = "catalog.unpublish"
)

// ModelCatalogAuditLog 记录谁在何时改了目录的哪一行、改前改后是什么。
// 隐藏状态决定模型对外是否可见，出问题时必须能回答"这是谁改的"。
type ModelCatalogAuditLog struct {
	Id      int    `json:"id"`
	ActorId int    `json:"actor_id" gorm:"type:int;not null;index"`
	Action  string `json:"action" gorm:"type:varchar(64);not null;index:idx_model_catalog_audit_action_time,priority:1"`
	// Model 受影响的模型标识；apply_seed 这类整体动作为空。
	Model       string `json:"model" gorm:"type:varchar(255);default:''"`
	Before      string `json:"before" gorm:"type:text"`
	After       string `json:"after" gorm:"type:text"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index:idx_model_catalog_audit_action_time,priority:2"`
}

func (l *ModelCatalogAuditLog) TableName() string {
	return "model_catalog_audit_log"
}

// RecordModelCatalogAudit 写目录审计记录；失败仅写系统日志，不影响主流程。
// before / after 为任意可序列化的快照，nil 表示该侧无内容。
func RecordModelCatalogAudit(actorId int, action, modelName string, before, after any) {
	log := &ModelCatalogAuditLog{
		ActorId:     actorId,
		Action:      action,
		Model:       modelName,
		Before:      marshalAuditSnapshot(before),
		After:       marshalAuditSnapshot(after),
		CreatedTime: utils.GetTimestamp(),
	}
	if err := DB.Create(log).Error; err != nil {
		logger.SysError("failed to record model catalog audit log: " + err.Error())
	}
}

func marshalAuditSnapshot(snapshot any) string {
	if snapshot == nil {
		return ""
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		logger.SysError("failed to encode model catalog audit snapshot: " + err.Error())
		return ""
	}
	return string(encoded)
}

// GetModelInfosByIds 按 id 批量取目录行，用于在写操作前后取快照。
func GetModelInfosByIds(ids []int) ([]*ModelInfo, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var infos []*ModelInfo
	if err := DB.Where("id IN ?", ids).Order("id asc").Find(&infos).Error; err != nil {
		return nil, err
	}
	return infos, nil
}
