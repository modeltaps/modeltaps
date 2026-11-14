package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Model access lists (ORG-15). An organization's effective model set has four layers, the most
// specific non-empty one deciding what a key may call:
//
//	key (inline list or attached named list)
//	→ member override (inline or named list), else the organization default (inline or named list)
//	→ the platform's approved models for the organization (set by the site admin only)
//	→ everything, when nothing is configured.
//
// Every layer must stay inside the layer above it: writes are rejected when an entry is not covered
// (ModelAllowlistCovers), and requests still check every layer, so a layer that is narrowed later
// never widens what lower layers can reach. Lists are allowlists only.

const (
	ModelAccessListNameMaxLen        = 100
	ModelAccessListDescriptionMaxLen = 500
	ModelAccessListMaxEntries        = 1000
	ModelAccessListMaxPerOrg         = 100
)

// ModelAccessList is a named, reusable allowlist of an organization. It can be attached to the
// organization default, to members and to keys; edits apply to every attachment at once. Restricted
// lists can only be attached to keys by organization admins.
type ModelAccessList struct {
	Id             int                         `json:"id"`
	OrganizationId int                         `json:"organization_id" gorm:"type:int;not null;index"`
	Name           string                      `json:"name" gorm:"type:varchar(100);not null"`
	Description    string                      `json:"description" gorm:"type:varchar(500);default:''"`
	Restricted     bool                        `json:"restricted" gorm:"default:false"`
	Models         datatypes.JSONSlice[string] `json:"models" gorm:"type:json"`
	CreatedBy      int                         `json:"created_by" gorm:"type:int;not null;default:0"`
	CreatedTime    int64                       `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64                       `json:"updated_time" gorm:"bigint"`
}

// ModelAccessListUsage counts where a list is attached.
type ModelAccessListUsage struct {
	OrgDefault bool `json:"org_default"`
	Members    int  `json:"members"`
	Tokens     int  `json:"tokens"`
}

// OrgModelAccessConflict names a lower layer that is no longer covered by a new approved-models list.
type OrgModelAccessConflict struct {
	Kind    string   `json:"kind"` // org_default | member | list
	Name    string   `json:"name"`
	Entries []string `json:"entries"`
}

func GetOrgModelAccessLists(organizationId int) ([]*ModelAccessList, error) {
	var lists []*ModelAccessList
	err := DB.Where("organization_id = ?", organizationId).Order("name asc, id asc").Find(&lists).Error
	return lists, err
}

func GetOrgModelAccessList(organizationId int, id int) (*ModelAccessList, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var list ModelAccessList
	if err := DB.Where("organization_id = ? AND id = ?", organizationId, id).First(&list).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

func errModelAccessListNotFound() error {
	return common.NewAPIError(common.APICodeModelAccessListNotFound, "the model access list does not exist", nil)
}

func errModelAccessUncovered(field string, entries []string) error {
	joined := strings.Join(entries, ", ")
	return common.NewAPIError(common.APICodeModelAccessNotCovered,
		fmt.Sprintf("%s includes models outside the allowed range: %s", field, joined),
		map[string]any{"field": field, "models": joined})
}

func validateAllowlistSize(entries []string) error {
	if len(entries) > ModelAccessListMaxEntries {
		return common.NewAPIError(common.APICodeModelAccessTooMany,
			fmt.Sprintf("a model list can hold at most %d entries", ModelAccessListMaxEntries),
			map[string]any{"max": ModelAccessListMaxEntries})
	}
	return nil
}

// validateModelAccessListInput normalizes and checks a named list against the organization's
// approved models.
func validateModelAccessListInput(org *Organization, list *ModelAccessList) error {
	list.Name = strings.TrimSpace(list.Name)
	list.Description = strings.TrimSpace(list.Description)
	if list.Name == "" || len([]rune(list.Name)) > ModelAccessListNameMaxLen {
		return common.NewAPIError(common.APICodeModelAccessListNameInvalid, "the list name is required and at most 100 characters", nil)
	}
	if len([]rune(list.Description)) > ModelAccessListDescriptionMaxLen {
		return common.NewAPIError(common.APICodeModelAccessListNameInvalid, "the description is at most 500 characters", nil)
	}
	models := NormalizeModelAllowlist(list.Models)
	if len(models) == 0 {
		return common.NewAPIError(common.APICodeModelAccessListEmpty, "a named list needs at least one model", nil)
	}
	if err := validateAllowlistSize(models); err != nil {
		return err
	}
	if missing := UncoveredModelAllowlistEntries(org.ApprovedModels, models); len(missing) > 0 {
		return errModelAccessUncovered("list", missing)
	}
	list.Models = models
	var count int64
	query := DB.Model(&ModelAccessList{}).Where("organization_id = ? AND LOWER(name) = ?", org.Id, strings.ToLower(list.Name))
	if list.Id > 0 {
		query = query.Where("id <> ?", list.Id)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return common.NewAPIError(common.APICodeModelAccessListNameDuplicate, "a list with this name already exists", nil)
	}
	return nil
}

func CreateOrgModelAccessList(org *Organization, list *ModelAccessList, operatorId int) error {
	list.Id = 0
	list.OrganizationId = org.Id
	if err := validateModelAccessListInput(org, list); err != nil {
		return err
	}
	var count int64
	if err := DB.Model(&ModelAccessList{}).Where("organization_id = ?", org.Id).Count(&count).Error; err != nil {
		return err
	}
	if count >= ModelAccessListMaxPerOrg {
		return common.NewAPIError(common.APICodeModelAccessTooMany,
			fmt.Sprintf("an organization can have at most %d lists", ModelAccessListMaxPerOrg),
			map[string]any{"max": ModelAccessListMaxPerOrg})
	}
	now := utils.GetTimestamp()
	list.CreatedBy = operatorId
	list.CreatedTime = now
	list.UpdatedTime = now
	if err := DB.Create(list).Error; err != nil {
		return err
	}
	invalidateOrgGuardrailCacheAllMembers(org)
	return nil
}

func UpdateOrgModelAccessList(org *Organization, list *ModelAccessList) error {
	existing, err := GetOrgModelAccessList(org.Id, list.Id)
	if err != nil {
		return errModelAccessListNotFound()
	}
	list.OrganizationId = org.Id
	if err := validateModelAccessListInput(org, list); err != nil {
		return err
	}
	existing.Name = list.Name
	existing.Description = list.Description
	existing.Restricted = list.Restricted
	existing.Models = list.Models
	existing.UpdatedTime = utils.GetTimestamp()
	if err := DB.Model(existing).Select("name", "description", "restricted", "models", "updated_time").Updates(existing).Error; err != nil {
		return err
	}
	*list = *existing
	invalidateOrgGuardrailCacheAllMembers(org)
	return nil
}

// DeleteOrgModelAccessList removes a list and detaches it everywhere; each attachment falls back to
// the next layer (the member's org default, the key's member layer). References are cleared in the
// same transaction so a later list can never inherit them through a reused id.
func DeleteOrgModelAccessList(org *Organization, id int) error {
	if _, err := GetOrgModelAccessList(org.Id, id); err != nil {
		return errModelAccessListNotFound()
	}
	var tokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("organization_id = ? AND id = ?", org.Id, id).Delete(&ModelAccessList{}).Error; err != nil {
			return err
		}
		var current Organization
		if err := tx.Select("id", "setting").Where("id = ?", org.Id).First(&current).Error; err != nil {
			return err
		}
		if setting := current.Setting.Data(); setting.ModelAccessListId == id {
			setting.ModelAccessListId = 0
			current.Setting.Set(setting)
			if err := tx.Model(&current).Select("setting").Updates(&current).Error; err != nil {
				return err
			}
			org.Setting.Set(setting)
		}
		var members []*OrganizationMember
		if err := tx.Where("organization_id = ?", org.Id).Find(&members).Error; err != nil {
			return err
		}
		for _, member := range members {
			setting := member.Setting.Data()
			if setting.ModelAccessListId != id {
				continue
			}
			setting.ModelAccessListId = 0
			member.Setting.Set(setting)
			if err := tx.Model(member).Select("setting").Updates(member).Error; err != nil {
				return err
			}
		}
		var tokens []*Token
		if err := tx.Where("user_id = ?", org.ShadowUserId).Find(&tokens).Error; err != nil {
			return err
		}
		for _, token := range tokens {
			setting := token.Setting.Data()
			if setting.Limits.LimitModelSetting.AccessListId != id {
				continue
			}
			setting.Limits.LimitModelSetting.AccessListId = 0
			token.Setting.Set(setting)
			if err := tx.Model(token).Select("setting").Updates(token).Error; err != nil {
				return err
			}
			tokenKeys = append(tokenKeys, token.Key)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if config.RedisEnabled {
		for _, key := range tokenKeys {
			if key != "" {
				redis.RedisDel(fmt.Sprintf(UserTokensKey, key))
			}
		}
	}
	invalidateOrgGuardrailCacheAllMembers(org)
	return nil
}

// GetOrgModelAccessListUsage counts the attachments of every list of the organization.
func GetOrgModelAccessListUsage(org *Organization) (map[int]*ModelAccessListUsage, error) {
	usage := map[int]*ModelAccessListUsage{}
	get := func(id int) *ModelAccessListUsage {
		if usage[id] == nil {
			usage[id] = &ModelAccessListUsage{}
		}
		return usage[id]
	}
	if id := org.Setting.Data().ModelAccessListId; id > 0 {
		get(id).OrgDefault = true
	}
	var members []*OrganizationMember
	if err := DB.Select("id", "setting").Where("organization_id = ?", org.Id).Find(&members).Error; err != nil {
		return nil, err
	}
	for _, member := range members {
		if id := member.Setting.Data().ModelAccessListId; id > 0 {
			get(id).Members++
		}
	}
	var tokens []*Token
	if err := DB.Select("id", "setting").Where("user_id = ?", org.ShadowUserId).Find(&tokens).Error; err != nil {
		return nil, err
	}
	for _, token := range tokens {
		if id := token.Setting.Data().Limits.LimitModelSetting.AccessListId; id > 0 {
			get(id).Tokens++
		}
	}
	return usage, nil
}

// orgModelAccessListMap loads the models of every list of the organization (for guardrails).
func orgModelAccessListMap(organizationId int) (map[int][]string, error) {
	lists, err := GetOrgModelAccessLists(organizationId)
	if err != nil {
		return nil, err
	}
	out := make(map[int][]string, len(lists))
	for _, list := range lists {
		out[list.Id] = []string(list.Models)
	}
	return out, nil
}

// resolveLayer returns the models of a layer: the attached list when it still exists, else the
// inline list.
func resolveLayer(lists map[int][]string, listId int, inline []string) []string {
	if listId > 0 {
		if models, ok := lists[listId]; ok {
			return models
		}
	}
	return inline
}

// orgDefaultAllowlist is the organization default layer (list or inline).
func orgDefaultAllowlist(org *Organization, lists map[int][]string) []string {
	setting := org.Setting.Data()
	return resolveLayer(lists, setting.ModelAccessListId, setting.ModelWhitelist)
}

// ValidateOrgDefaultModelAccess checks a new organization default (inline list or named list id).
func ValidateOrgDefaultModelAccess(org *Organization, listId int, inline []string) ([]string, error) {
	return validateOrgLayer(org, "org_default", listId, inline)
}

// ValidateMemberModelAccess checks a new member override (inline list or named list id).
func ValidateMemberModelAccess(org *Organization, listId int, inline []string) ([]string, error) {
	return validateOrgLayer(org, "member", listId, inline)
}

// validateOrgLayer: a layer below the approved models uses either a named list (already inside the
// approved models) or an inline list that must be covered by them. Returns the normalized inline list.
func validateOrgLayer(org *Organization, field string, listId int, inline []string) ([]string, error) {
	if listId > 0 {
		if _, err := GetOrgModelAccessList(org.Id, listId); err != nil {
			return nil, errModelAccessListNotFound()
		}
		return []string{}, nil
	}
	models := NormalizeModelAllowlist(inline)
	if err := validateAllowlistSize(models); err != nil {
		return nil, err
	}
	if missing := UncoveredModelAllowlistEntries(org.ApprovedModels, models); len(missing) > 0 {
		return nil, errModelAccessUncovered(field, missing)
	}
	return models, nil
}

// UpdateOrgDefaultModelAccess saves the organization default layer.
func UpdateOrgDefaultModelAccess(org *Organization, listId int, inline []string) error {
	models, err := ValidateOrgDefaultModelAccess(org, listId, inline)
	if err != nil {
		return err
	}
	setting := org.Setting.Data()
	setting.ModelAccessListId = listId
	setting.ModelWhitelist = models
	org.Setting.Set(setting)
	if err := org.UpdateSetting(); err != nil {
		return err
	}
	invalidateOrgGuardrailCacheAllMembers(org)
	return nil
}

// ValidateOrgTokenModelAccess checks the model limit of an organization key against the layers above
// it: the approved models and the effective member layer of the key's creator. Restricted lists can
// only be attached by organization admins (operatorRole).
func ValidateOrgTokenModelAccess(org *Organization, creatorUserId int, operatorRole string, limit *LimitModelSetting) error {
	*limit = normalizeLimitModelSetting(*limit)
	if limit.AccessListId == 0 && len(limit.Models) == 0 {
		return nil // no key layer: the layers above apply as they are
	}
	if err := validateAllowlistSize(limit.Models); err != nil {
		return err
	}
	member, err := GetOrganizationMember(org.Id, creatorUserId)
	if err != nil {
		return common.NewAPIError(common.APICodeOrgUserNotMember, "this user is not a member of the organization", nil)
	}
	lists, err := orgModelAccessListMap(org.Id)
	if err != nil {
		return err
	}
	entries := limit.Models
	if limit.AccessListId > 0 {
		list, err := GetOrgModelAccessList(org.Id, limit.AccessListId)
		if err != nil {
			return errModelAccessListNotFound()
		}
		if list.Restricted && OrgRoleWeight(operatorRole) < OrgRoleWeight(OrgRoleAdmin) {
			return common.NewAPIError(common.APICodeModelAccessListRestricted, "only organization admins can attach a restricted list", nil)
		}
		entries = []string(list.Models)
	}
	memberLayer := memberAllowlist(org, member, lists)
	var missing []string
	for _, entry := range entries {
		if !ModelAllowlistCovers(org.ApprovedModels, entry) || !ModelAllowlistCovers(memberLayer, entry) {
			missing = append(missing, entry)
		}
	}
	if len(missing) > 0 {
		return errModelAccessUncovered("token", missing)
	}
	return nil
}

// memberAllowlist is the member layer: the member override when set, else the organization default.
func memberAllowlist(org *Organization, member *OrganizationMember, lists map[int][]string) []string {
	setting := member.Setting.Data()
	if models := resolveLayer(lists, setting.ModelAccessListId, setting.ModelWhitelist); len(models) > 0 {
		return models
	}
	return orgDefaultAllowlist(org, lists)
}

// SetOrganizationApprovedModels saves the platform's approved models of an organization (site admin).
// It is rejected while a lower org-level layer (default, member override, named list) still uses an
// entry the new list does not cover; keys are checked at request time and shown as out of range.
func SetOrganizationApprovedModels(org *Organization, models []string) ([]OrgModelAccessConflict, error) {
	models = NormalizeModelAllowlist(models)
	if err := validateAllowlistSize(models); err != nil {
		return nil, err
	}
	conflicts, err := orgModelAccessConflicts(org, models)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		return conflicts, common.NewAPIError(common.APICodeModelAccessCeilingInUse,
			"some lists of the organization still use models outside the new approved models", nil)
	}
	org.ApprovedModels = models
	org.UpdatedTime = utils.GetTimestamp()
	if err := DB.Model(org).Select("approved_models", "updated_time").Updates(org).Error; err != nil {
		return nil, err
	}
	invalidateOrgGuardrailCacheAllMembers(org)
	return nil, nil
}

func orgModelAccessConflicts(org *Organization, ceiling []string) ([]OrgModelAccessConflict, error) {
	var conflicts []OrgModelAccessConflict
	if len(ceiling) == 0 {
		return conflicts, nil
	}
	if missing := UncoveredModelAllowlistEntries(ceiling, org.Setting.Data().ModelWhitelist); len(missing) > 0 {
		conflicts = append(conflicts, OrgModelAccessConflict{Kind: "org_default", Entries: missing})
	}
	lists, err := GetOrgModelAccessLists(org.Id)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		if missing := UncoveredModelAllowlistEntries(ceiling, list.Models); len(missing) > 0 {
			conflicts = append(conflicts, OrgModelAccessConflict{Kind: "list", Name: list.Name, Entries: missing})
		}
	}
	var members []*OrganizationMember
	if err := DB.Where("organization_id = ?", org.Id).Find(&members).Error; err != nil {
		return nil, err
	}
	for _, member := range members {
		missing := UncoveredModelAllowlistEntries(ceiling, member.Setting.Data().ModelWhitelist)
		if len(missing) == 0 {
			continue
		}
		name := fmt.Sprintf("#%d", member.UserId)
		if user, err := GetUserById(member.UserId, false); err == nil && user != nil {
			name = user.Username
		}
		conflicts = append(conflicts, OrgModelAccessConflict{Kind: "member", Name: name, Entries: missing})
	}
	return conflicts, nil
}

// OrgModelAccessScope is what the console needs to build model pickers for one member: the
// approved models, the organization default and the member layer (empty = not restricted).
type OrgModelAccessScope struct {
	ApprovedModels  []string `json:"approved_models"`
	DefaultListId   int      `json:"default_list_id"`
	DefaultModels   []string `json:"default_models"`
	MemberListId    int      `json:"member_list_id"`
	MemberModels    []string `json:"member_models"`
	EffectiveMember []string `json:"effective_member"`
}

func GetOrgModelAccessScope(org *Organization, memberUserId int) (*OrgModelAccessScope, error) {
	lists, err := orgModelAccessListMap(org.Id)
	if err != nil {
		return nil, err
	}
	setting := org.Setting.Data()
	scope := &OrgModelAccessScope{
		ApprovedModels: nonNil(org.ApprovedModels),
		DefaultListId:  setting.ModelAccessListId,
		DefaultModels:  nonNil(setting.ModelWhitelist),
		MemberModels:   []string{},
	}
	scope.EffectiveMember = nonNil(orgDefaultAllowlist(org, lists))
	if memberUserId > 0 {
		member, err := GetOrganizationMember(org.Id, memberUserId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, common.NewAPIError(common.APICodeOrgUserNotMember, "this user is not a member of the organization", nil)
			}
			return nil, err
		}
		memberSetting := member.Setting.Data()
		scope.MemberListId = memberSetting.ModelAccessListId
		scope.MemberModels = nonNil(memberSetting.ModelWhitelist)
		scope.EffectiveMember = nonNil(memberAllowlist(org, member, lists))
	}
	return scope, nil
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// LimitModelSettingChanged reports whether a key's model limit differs after normalization.
func LimitModelSettingChanged(before, after LimitModelSetting) bool {
	a := normalizeLimitModelSetting(before)
	b := normalizeLimitModelSetting(after)
	if a.AccessListId != b.AccessListId || len(a.Models) != len(b.Models) {
		return true
	}
	for i := range a.Models {
		if !strings.EqualFold(a.Models[i], b.Models[i]) {
			return true
		}
	}
	return false
}
