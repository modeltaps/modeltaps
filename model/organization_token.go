package model

// GetOrgTokensList 组织令牌列表:令牌归属组织影子账户(tokens.user_id = shadowUserId)。
// createdBy > 0 时仅返回该成员创建的令牌(Member 自见);0 表示不过滤(Owner/Admin 全见)。
func GetOrgTokensList(shadowUserId int, createdBy int, params *GenericParams) (*DataResult[Token], error) {
	var tokens []*Token
	db := DB.Where("user_id = ?", shadowUserId)
	if createdBy > 0 {
		db = db.Where("created_by = ?", createdBy)
	}

	if params.Keyword != "" {
		db = db.Where("name LIKE ?", params.Keyword+"%")
	}

	return PaginateAndOrder(db, &params.PaginationParams, &tokens, allowedTokenOrderFields)
}
