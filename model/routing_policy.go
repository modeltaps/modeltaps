package model

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ModelInfoKindRoutingPolicy 标记目录里由路由策略联动维护的那一行。
const ModelInfoKindRoutingPolicy = "routing_policy"

// 路由策略的取值边界。
const (
	RoutingPolicyMinRoutes         = 2
	RoutingPolicyMaxRoutes         = 16
	RoutingPolicyDefaultThreshold  = 0.5
	RoutingPolicyDefaultPinTTL     = 3600
	RoutingPolicyMaxPinTTL         = 7 * 24 * 3600
	RoutingPolicyDefaultInputChars = 4000
	RoutingPolicyMaxInputChars     = 200000
	RoutingPolicyDefaultTimeoutMs  = 3000
	RoutingPolicyMaxTimeoutMs      = 60000
	routingPolicyMaxLabelLen       = 64
	routingPolicyMaxNameLen        = 100
)

// 决策的打分方式。
const (
	// RoutingScoringAuto 先用 logprobs 打分；决策模型或渠道不返回 logprobs 时改用自报置信度。
	RoutingScoringAuto = "auto"
	// RoutingScoringLogprobs 只用 logprobs：各标签的概率来自模型对选项字母的输出概率。
	RoutingScoringLogprobs = "logprobs"
	// RoutingScoringSelfReported 让模型用 JSON 回答标签和它自报的置信度。
	RoutingScoringSelfReported = "self_reported"
)

// RoutingRoute 策略里的一条路由：决策模型选中 Label 时请求转发给 TargetModel。
type RoutingRoute struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	TargetModel string `json:"target_model"`
}

// RoutingPolicy 自动路由策略：对外是一个模型名（Name），请求进来后把会话首条用户消息交给
// DecisionModel 在 Routes 的标签里做单选，置信度达到 Threshold 就转发给对应目标，否则走
// FallbackModel。决策按会话固定 PinTTLSeconds 秒。目录里同时有一行 kind=routing_policy 的
// model_info，使策略能出现在模型列表、被白名单引用、被隐藏。
type RoutingPolicy struct {
	Id int `json:"id"`
	// OrganizationId and UserId are the owning scope (see RoutingScope): both 0 for platform policies
	// (managed by system admins, usable by everyone), the organization for organization policies
	// (usable by its API keys), the user for personal policies (usable by that user's own API keys).
	// Names are unique within one scope; an own policy may reuse a platform policy's name and then
	// wins for its scope.
	OrganizationId int    `json:"organization_id" gorm:"not null;default:0;uniqueIndex:idx_routing_policies_scope_name,priority:1"`
	UserId         int    `json:"user_id" gorm:"not null;default:0;uniqueIndex:idx_routing_policies_scope_name,priority:2"`
	Name           string `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_routing_policies_scope_name,priority:3"`
	DisplayName string `json:"display_name" gorm:"type:varchar(100);default:''"`
	Description string `json:"description" gorm:"type:text"`
	// Enabled 不带 gorm 默认值：带了 default:true 时 false 会被当零值替换成 true。
	// 请求里省略 enabled 时的「默认启用」由控制器补。
	Enabled bool `json:"enabled"`
	// DecisionModel 做分类的模型；Question 是给它的问题（含少量上下文，不写长指令）。
	DecisionModel string                            `json:"decision_model" gorm:"type:varchar(255);not null"`
	Question      string                            `json:"question" gorm:"type:text"`
	Routes        datatypes.JSONSlice[RoutingRoute] `json:"routes" gorm:"type:json"`
	FallbackModel string                            `json:"fallback_model" gorm:"type:varchar(255);not null"`
	// Scoring 打分方式（RoutingScoring*）；存量行为空，Normalize 补为 auto。
	Scoring string `json:"scoring" gorm:"type:varchar(20);default:''"`
	// 下面几个数值字段都不带 gorm 默认值：0 是合法取值（阈值 0、TTL 0 = 每次分类），
	// gorm 的 default 会把零值改写成默认值。Normalize 只给「0 = 用默认」的字段补值。
	// Threshold 0~1：最高标签概率低于它走兜底。
	Threshold float64 `json:"threshold"`
	// PinTTLSeconds 会话固定时长，0 = 每次请求都重新分类。
	PinTTLSeconds int `json:"pin_ttl_seconds"`
	// MaxInputChars 送给决策模型的首条消息截断长度，0 = 默认值。
	MaxInputChars int `json:"max_input_chars"`
	// DecisionTimeoutMs 决策调用超时，0 = 默认值；超时走兜底。
	DecisionTimeoutMs int   `json:"decision_timeout_ms"`
	CreatedAt         int64 `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (p *RoutingPolicy) TableName() string {
	return "routing_policies"
}

// RoutingPolicyCallPrefix is how API keys call their own (organization or personal) policies:
// model "policy/<name>". Platform policies are called by their bare name.
const RoutingPolicyCallPrefix = "policy/"

// RoutingScope is the owner of a policy: the platform (zero value), an organization or a user.
type RoutingScope struct {
	OrganizationId int
	UserId         int
}

// PlatformRoutingScope is the scope of the policies system admins manage.
var PlatformRoutingScope = RoutingScope{}

func OrganizationRoutingScope(organizationId int) RoutingScope {
	return RoutingScope{OrganizationId: organizationId}
}

func PersonalRoutingScope(userId int) RoutingScope {
	return RoutingScope{UserId: userId}
}

func (s RoutingScope) IsPlatform() bool { return s.OrganizationId == 0 && s.UserId == 0 }

// Scope returns the policy's owner.
func (p *RoutingPolicy) Scope() RoutingScope {
	return RoutingScope{OrganizationId: p.OrganizationId, UserId: p.UserId}
}

// IsOwnPolicy reports whether the policy belongs to an organization or a user rather than the platform.
func (p *RoutingPolicy) IsOwnPolicy() bool {
	return !p.Scope().IsPlatform()
}

// CallName is the model name API keys use for the policy.
func (p *RoutingPolicy) CallName() string {
	if p.IsOwnPolicy() {
		return RoutingPolicyCallPrefix + p.Name
	}
	return p.Name
}

// routingPolicyNamePattern 策略名会作为模型名进 URL（Gemini 的 models/<name>）与各处选择器，
// 只放行字母、数字与 . _ / - ，且不能以 / 开头或结尾。
var routingPolicyNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

var (
	ErrRoutingPolicyNotFound         = common.NewAPIError(common.APICodeRoutingPolicyNotFound, "routing policy not found", nil)
	ErrRoutingPolicyNameInvalid      = common.NewAPIError(common.APICodeRoutingPolicyNameInvalid, "name may only contain letters, digits and . _ / -, up to 100 characters, must not start or end with / and must not start with policy/", nil)
	ErrRoutingPolicyNameTaken        = common.NewAPIError(common.APICodeRoutingPolicyNameTaken, "name is already used by a model or another policy", nil)
	ErrRoutingPolicyRoutesCount      = common.NewAPIError(common.APICodeRoutingPolicyRoutesCount, "a policy needs between 2 and 16 routes", map[string]any{"min": RoutingPolicyMinRoutes, "max": RoutingPolicyMaxRoutes})
	ErrRoutingPolicyThresholdInvalid = common.NewAPIError(common.APICodeRoutingPolicyThresholdInvalid, "threshold must be between 0 and 1", nil)
	ErrRoutingPolicyScoringInvalid   = common.NewAPIError(common.APICodeRoutingPolicyScoringInvalid, "scoring must be auto, logprobs or self_reported", nil)
	ErrRoutingPolicyReferenced       = common.NewAPIError(common.APICodeRoutingPolicyReferenced, "policy is referenced by model aliases", nil)
	ErrOrgRoutingPolicyDisabled      = common.NewAPIError(common.APICodeOrgRoutingPolicyDisabled, "organization routing policies are not enabled", nil)
)

// routingPolicyModelUnavailable a model an own policy refers to is hidden or has no channel in the
// scope's group.
func routingPolicyModelUnavailable(field, name string) error {
	return common.NewAPIError(common.APICodeRoutingPolicyModelUnavailable, field+" is not available to this organization: "+name, map[string]any{"field": field, "model": name})
}

func routingPolicyCountExceeded(max int) error {
	return common.NewAPIError(common.APICodeRoutingPolicyCountExceeded, "this organization already has the maximum number of routing policies", map[string]any{"max": max})
}

func routingPolicyModelRequired(field string) error {
	return common.NewAPIError(common.APICodeRoutingPolicyModelRequired, field+" is required", map[string]any{"field": field})
}

// ErrRoutingPolicyModelIsPolicy 某个字段填了策略名而不是模型。
func ErrRoutingPolicyModelIsPolicy(field, name string) error {
	return routingPolicyModelIsPolicy(field, name)
}

func routingPolicyModelIsPolicy(field, name string) error {
	return common.NewAPIError(common.APICodeRoutingPolicyModelIsPolicy, field+" must be a model, not a routing policy: "+name, map[string]any{"field": field, "model": name})
}

func routingPolicyLabelInvalid(index int) error {
	return common.NewAPIError(common.APICodeRoutingPolicyLabelInvalid, "route label is required and at most 64 characters", map[string]any{"index": index})
}

func routingPolicyLabelDuplicate(label string) error {
	return common.NewAPIError(common.APICodeRoutingPolicyLabelDuplicate, "duplicate route label: "+label, map[string]any{"label": label})
}

func routingPolicyLimitInvalid(field string, max int) error {
	return common.NewAPIError(common.APICodeRoutingPolicyLimitInvalid, field+" is out of range", map[string]any{"field": field, "max": max})
}

// ValidateRoutingPolicyName 校验策略名格式；是否与现有模型 / 策略冲突由 Validate 另查。
func ValidateRoutingPolicyName(name string) error {
	if name == "" || len(name) > routingPolicyMaxNameLen || !routingPolicyNamePattern.MatchString(name) || strings.HasSuffix(name, "/") ||
		strings.HasPrefix(strings.ToLower(name), RoutingPolicyCallPrefix) {
		return ErrRoutingPolicyNameInvalid
	}
	return nil
}

// Normalize 去掉各字段首尾空白，并给 0 值的可选参数补默认值。校验前调用。
func (p *RoutingPolicy) Normalize() {
	p.Name = strings.TrimSpace(p.Name)
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.Description = strings.TrimSpace(p.Description)
	p.DecisionModel = strings.TrimSpace(p.DecisionModel)
	p.Question = strings.TrimSpace(p.Question)
	p.FallbackModel = strings.TrimSpace(p.FallbackModel)
	for i := range p.Routes {
		p.Routes[i].Label = strings.TrimSpace(p.Routes[i].Label)
		p.Routes[i].Description = strings.TrimSpace(p.Routes[i].Description)
		p.Routes[i].TargetModel = strings.TrimSpace(p.Routes[i].TargetModel)
	}
	p.Scoring = strings.TrimSpace(p.Scoring)
	if p.Scoring == "" {
		p.Scoring = RoutingScoringAuto
	}
	if p.MaxInputChars == 0 {
		p.MaxInputChars = RoutingPolicyDefaultInputChars
	}
	if p.DecisionTimeoutMs == 0 {
		p.DecisionTimeoutMs = RoutingPolicyDefaultTimeoutMs
	}
}

// Validate 校验一条策略。isPolicy 判断某个模型名是否是（另一条）策略，决策模型、目标与兜底都
// 不能是策略（禁止嵌套）；策略自身的名字也算。
func (p *RoutingPolicy) Validate(isPolicy func(name string) bool) error {
	if err := ValidateRoutingPolicyName(p.Name); err != nil {
		return err
	}
	if len(p.DisplayName) > routingPolicyMaxNameLen {
		return routingPolicyLimitInvalid("display_name", routingPolicyMaxNameLen)
	}
	notPolicy := func(field, name string) error {
		if name == "" {
			return routingPolicyModelRequired(field)
		}
		if name == p.Name || (isPolicy != nil && isPolicy(name)) {
			return routingPolicyModelIsPolicy(field, name)
		}
		return nil
	}
	if err := notPolicy("decision_model", p.DecisionModel); err != nil {
		return err
	}
	if err := notPolicy("fallback_model", p.FallbackModel); err != nil {
		return err
	}
	if len(p.Routes) < RoutingPolicyMinRoutes || len(p.Routes) > RoutingPolicyMaxRoutes {
		return ErrRoutingPolicyRoutesCount
	}
	seen := make(map[string]bool, len(p.Routes))
	for i, route := range p.Routes {
		if route.Label == "" || len(route.Label) > routingPolicyMaxLabelLen {
			return routingPolicyLabelInvalid(i)
		}
		key := strings.ToLower(route.Label)
		if seen[key] {
			return routingPolicyLabelDuplicate(route.Label)
		}
		seen[key] = true
		if err := notPolicy("routes[].target_model", route.TargetModel); err != nil {
			return err
		}
	}
	switch p.Scoring {
	case RoutingScoringAuto, RoutingScoringLogprobs, RoutingScoringSelfReported:
	default:
		return ErrRoutingPolicyScoringInvalid
	}
	if p.Threshold < 0 || p.Threshold > 1 {
		return ErrRoutingPolicyThresholdInvalid
	}
	if p.PinTTLSeconds < 0 || p.PinTTLSeconds > RoutingPolicyMaxPinTTL {
		return routingPolicyLimitInvalid("pin_ttl_seconds", RoutingPolicyMaxPinTTL)
	}
	if p.MaxInputChars < 0 || p.MaxInputChars > RoutingPolicyMaxInputChars {
		return routingPolicyLimitInvalid("max_input_chars", RoutingPolicyMaxInputChars)
	}
	if p.DecisionTimeoutMs < 0 || p.DecisionTimeoutMs > RoutingPolicyMaxTimeoutMs {
		return routingPolicyLimitInvalid("decision_timeout_ms", RoutingPolicyMaxTimeoutMs)
	}
	return nil
}

// RouteFor 按标签找路由（忽略大小写），找不到返回 nil。
func (p *RoutingPolicy) RouteFor(label string) *RoutingRoute {
	for i := range p.Routes {
		if strings.EqualFold(p.Routes[i].Label, label) {
			return &p.Routes[i]
		}
	}
	return nil
}

// TargetModels 返回策略可能转发到的所有模型（各路由目标 + 兜底），去重、保持出现顺序。
func (p *RoutingPolicy) TargetModels() []string {
	seen := make(map[string]bool, len(p.Routes)+1)
	targets := make([]string, 0, len(p.Routes)+1)
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		targets = append(targets, name)
	}
	for _, route := range p.Routes {
		add(route.TargetModel)
	}
	add(p.FallbackModel)
	return targets
}

// catalogRow 生成（或刷新）与策略联动的目录行。Locked 让价格 / 元信息同步跳过它。
func (p *RoutingPolicy) catalogRow(existing *ModelInfo) *ModelInfo {
	row := existing
	if row == nil {
		row = &ModelInfo{
			Mode:      ModelModeChat,
			Endpoints: `["` + ModelEndpointChat + `","` + ModelEndpointResponses + `"]`,
		}
	}
	row.Model = p.Name
	row.Name = p.DisplayName
	if row.Name == "" {
		row.Name = p.Name
	}
	row.Description = p.Description
	row.Kind = ModelInfoKindRoutingPolicy
	row.Source = ModelInfoSourceManual
	row.Locked = true
	return row
}

// ---- 查询 ----

// IsOrganizationPolicy reports whether the policy belongs to an organization.
func (p *RoutingPolicy) IsOrganizationPolicy() bool {
	return p.OrganizationId > 0
}

// IsPersonalPolicy reports whether the policy belongs to a single user.
func (p *RoutingPolicy) IsPersonalPolicy() bool {
	return p.UserId > 0
}

// GetRoutingPolicies returns every policy of every scope.
func GetRoutingPolicies() ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Order("id asc").Find(&policies).Error
	return policies, err
}

// GetRoutingPoliciesByScope returns the policies of one scope.
func GetRoutingPoliciesByScope(scope RoutingScope) ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Where("organization_id = ? AND user_id = ?", scope.OrganizationId, scope.UserId).Order("id asc").Find(&policies).Error
	return policies, err
}

// GetAllPersonalRoutingPolicies returns every user's personal policies, by user then id.
func GetAllPersonalRoutingPolicies() ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Where("user_id > ?", 0).Order("user_id asc, id asc").Find(&policies).Error
	return policies, err
}

// GetAllOrganizationRoutingPolicies returns every organization's policies, by organization then id.
func GetAllOrganizationRoutingPolicies() ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Where("organization_id > ?", 0).Order("organization_id asc, id asc").Find(&policies).Error
	return policies, err
}

func GetRoutingPolicyById(id int) (*RoutingPolicy, error) {
	policy := &RoutingPolicy{}
	err := DB.First(policy, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoutingPolicyNotFound
	}
	return policy, err
}

// GetScopedRoutingPolicyById returns the policy only when it belongs to the scope, so one
// organization or user can never read or change another's policy by guessing an id.
func GetScopedRoutingPolicyById(scope RoutingScope, id int) (*RoutingPolicy, error) {
	policy, err := GetRoutingPolicyById(id)
	if err != nil {
		return nil, err
	}
	if policy.Scope() != scope {
		return nil, ErrRoutingPolicyNotFound
	}
	return policy, nil
}

// GetOrganizationRoutingPolicyById is GetScopedRoutingPolicyById for an organization.
func GetOrganizationRoutingPolicyById(organizationId, id int) (*RoutingPolicy, error) {
	return GetScopedRoutingPolicyById(OrganizationRoutingScope(organizationId), id)
}

// GetRoutingPolicyByName looks up a platform policy by name.
func GetRoutingPolicyByName(name string) (*RoutingPolicy, error) {
	return getRoutingPolicyByScopedName(PlatformRoutingScope, name)
}

func getRoutingPolicyByScopedName(scope RoutingScope, name string) (*RoutingPolicy, error) {
	policy := &RoutingPolicy{}
	err := DB.First(policy, "organization_id = ? AND user_id = ? AND name = ?", scope.OrganizationId, scope.UserId, name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoutingPolicyNotFound
	}
	return policy, err
}

func countScopedRoutingPolicies(scope RoutingScope) (int64, error) {
	var count int64
	err := DB.Model(&RoutingPolicy{}).Where("organization_id = ? AND user_id = ?", scope.OrganizationId, scope.UserId).Count(&count).Error
	return count, err
}

// maxCount is the cap on the number of policies of an own scope (0 = no cap).
func (s RoutingScope) maxCount() int {
	if s.OrganizationId > 0 {
		return config.OrganizationRoutingPolicyMaxCount
	}
	return config.PersonalRoutingPolicyMaxCount
}

// routingPolicyNameTaken 判断名字是否已被目录里的模型（含别名）或同一作用域里的另一条策略占用。
// 渠道提供的模型未必有目录行，所以还要问 balancer；它没加载时视为未占用。
// An own policy may share a platform policy's name (it overrides it for its scope), so the catalog
// row a platform policy owns does not count against it.
func routingPolicyNameTaken(scope RoutingScope, name string, selfId int) (bool, error) {
	if info, err := GetModelInfoByModel(name); err == nil && info != nil {
		if info.Kind != ModelInfoKindRoutingPolicy {
			return true, nil
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if other, err := getRoutingPolicyByScopedName(scope, name); err == nil && other.Id != selfId {
		return true, nil
	} else if err != nil && !errors.Is(err, ErrRoutingPolicyNotFound) {
		return false, err
	}
	if ChannelGroup.HasModel(name) {
		return true, nil
	}
	return false, nil
}

// isOtherRoutingPolicy reports whether a name refers to a policy other than selfId that the policy
// could see: platform policies for everyone, plus the scope's own policies (by bare name or
// policy/<name>). Policies must not point at policies, so this is the "no nesting" check of Validate.
func isOtherRoutingPolicy(scope RoutingScope, selfId int) func(name string) bool {
	return func(name string) bool {
		if other := LookupRoutingPolicy(name); other != nil && other.Id != selfId {
			return true
		}
		if !scope.IsPlatform() {
			bare := strings.TrimPrefix(name, RoutingPolicyCallPrefix)
			if other := lookupScopedRoutingPolicy(scope, bare); other != nil && other.Id != selfId {
				return true
			}
		}
		return strings.HasPrefix(name, RoutingPolicyCallPrefix)
	}
}

// IsRoutingPolicyModelFor is the nesting check for drafts of the given scope.
func IsRoutingPolicyModelFor(scope RoutingScope) func(name string) bool {
	return isOtherRoutingPolicy(scope, 0)
}

// ValidateScopedModels checks that every model an own policy refers to can be used by its scope: it
// is not hidden in the catalog and the scope's group has a channel for it. Platform admins hide
// models they only want reachable through their own policies; an own policy must not reach them.
func (p *RoutingPolicy) ValidateScopedModels(scope RoutingScope) error {
	group, err := scopeRoutingGroup(scope)
	if err != nil {
		return err
	}
	check := func(field, name string) error {
		return checkScopedRoutingModel(group, field, name)
	}
	if err := check("decision_model", p.DecisionModel); err != nil {
		return err
	}
	if err := check("fallback_model", p.FallbackModel); err != nil {
		return err
	}
	for _, route := range p.Routes {
		if err := check("routes[].target_model", route.TargetModel); err != nil {
			return err
		}
	}
	return nil
}

// CheckScopedRoutingModel is ValidateScopedModels for a single model (used by the decision model
// probe of the organization and personal consoles).
func CheckScopedRoutingModel(scope RoutingScope, field, name string) error {
	group, err := scopeRoutingGroup(scope)
	if err != nil {
		return err
	}
	return checkScopedRoutingModel(group, field, name)
}

func checkScopedRoutingModel(group, field, name string) error {
	if !RoutingTargetUsable(name) {
		return routingPolicyModelUnavailable(field, name)
	}
	canonical, _ := ResolveCanonicalModel(name)
	if _, err := ChannelGroup.GetMatchedModelName(group, canonical); err != nil {
		return routingPolicyModelUnavailable(field, name)
	}
	return nil
}

// ScopedRoutingModels lists the models an own policy may use, for the console's model pickers:
// routable in the scope's group, visible in the catalog, not an alias and not a policy. Sorted by name.
func ScopedRoutingModels(scope RoutingScope) ([]string, error) {
	group, err := scopeRoutingGroup(scope)
	if err != nil {
		return nil, err
	}
	models, err := ChannelGroup.GetGroupModels(group)
	if err != nil {
		return []string{}, nil
	}
	// Targets of an organization's policies stay inside its approved models (ORG-15).
	var approved []string
	if scope.OrganizationId > 0 {
		var org Organization
		if err := DB.Select("id", "approved_models").Where("id = ?", scope.OrganizationId).First(&org).Error; err == nil {
			approved = []string(org.ApprovedModels)
		}
	}
	isPolicy := IsRoutingPolicyModelFor(scope)
	usable := make([]string, 0, len(models))
	for _, name := range models {
		if strings.Contains(name, "*") || isPolicy(name) || !IsCatalogVisible(name) || !ModelAllowlistPermits(approved, name) {
			continue
		}
		usable = append(usable, name)
	}
	sort.Strings(usable)
	return usable, nil
}

// RoutingTargetUsable reports whether an own policy may send a request to the model: hidden catalog
// models are off limits (checked again at request time, since the platform may hide a model after
// the policy was saved).
func RoutingTargetUsable(name string) bool {
	canonical, _ := ResolveCanonicalModel(name)
	return !(config.CatalogEnforceHidden() && IsModelHidden(canonical))
}

// scopeRoutingGroup returns the user group the scope's API keys route in by default: the group of
// the organization's shadow account, or the user's own group.
func scopeRoutingGroup(scope RoutingScope) (string, error) {
	if scope.OrganizationId > 0 {
		org, err := GetOrganizationById(scope.OrganizationId)
		if err != nil {
			return "", err
		}
		return CacheGetUserGroup(org.ShadowUserId)
	}
	return CacheGetUserGroup(scope.UserId)
}

// ---- 写入 ----

// validateForWrite runs the checks shared by Insert and Update.
func (p *RoutingPolicy) validateForWrite() error {
	p.Normalize()
	if p.OrganizationId > 0 && p.UserId > 0 {
		return ErrRoutingPolicyNotFound
	}
	if err := p.Validate(isOtherRoutingPolicy(p.Scope(), p.Id)); err != nil {
		return err
	}
	if p.IsOwnPolicy() {
		return p.ValidateScopedModels(p.Scope())
	}
	return nil
}

// Insert 校验并写入策略；平台策略在同一事务里建目录行。Own (organization and personal) policies
// get no catalog row: the catalog is global and would show the policy to everyone else.
func (p *RoutingPolicy) Insert() error {
	if err := p.validateForWrite(); err != nil {
		return err
	}
	if p.IsOwnPolicy() {
		count, err := countScopedRoutingPolicies(p.Scope())
		if err != nil {
			return err
		}
		if max := p.Scope().maxCount(); max > 0 && count >= int64(max) {
			return routingPolicyCountExceeded(max)
		}
	}
	taken, err := routingPolicyNameTaken(p.Scope(), p.Name, 0)
	if err != nil {
		return err
	}
	if taken {
		return ErrRoutingPolicyNameTaken
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		if p.IsOwnPolicy() {
			return nil
		}
		return tx.Create(p.catalogRow(nil)).Error
	})
	if err != nil {
		return err
	}
	refreshRoutingPolicyCaches()
	return nil
}

// Update 校验并整行保存策略；改名时目录行跟着改名，被别名引用的旧名不允许改。
// The owning scope never changes: a policy cannot move between the platform, organizations and users.
func (p *RoutingPolicy) Update() error {
	current, err := GetRoutingPolicyById(p.Id)
	if err != nil {
		return err
	}
	p.OrganizationId, p.UserId = current.OrganizationId, current.UserId
	if err := p.validateForWrite(); err != nil {
		return err
	}
	if p.Name != current.Name {
		taken, err := routingPolicyNameTaken(p.Scope(), p.Name, p.Id)
		if err != nil {
			return err
		}
		if taken {
			return ErrRoutingPolicyNameTaken
		}
		if !p.IsOwnPolicy() {
			refs, err := ModelAliasReferences(current.Name)
			if err != nil {
				return err
			}
			if len(refs) > 0 {
				return routingPolicyReferenced(refs)
			}
		}
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("created_at").Save(p).Error; err != nil {
			return err
		}
		if p.IsOwnPolicy() {
			return nil
		}
		existing := &ModelInfo{}
		findErr := tx.Where("model = ? AND kind = ?", current.Name, ModelInfoKindRoutingPolicy).First(existing).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return tx.Create(p.catalogRow(nil)).Error
		}
		if findErr != nil {
			return findErr
		}
		return tx.Omit("created_at", "synced_at", "supported_voices").Save(p.catalogRow(existing)).Error
	})
	if err != nil {
		return err
	}
	refreshRoutingPolicyCaches()
	return nil
}

func routingPolicyReferenced(refs []string) error {
	return common.NewAPIError(common.APICodeRoutingPolicyReferenced, "policy is referenced by model aliases: "+strings.Join(refs, ", "), map[string]any{"aliases": strings.Join(refs, ", ")})
}

// SetRoutingPolicyEnabled 只切换启用状态，不触碰其它字段。
func SetRoutingPolicyEnabled(id int, enabled bool) (*RoutingPolicy, error) {
	policy, err := GetRoutingPolicyById(id)
	if err != nil {
		return nil, err
	}
	if err := DB.Model(&RoutingPolicy{}).Where("id = ?", id).Update("enabled", enabled).Error; err != nil {
		return nil, err
	}
	policy.Enabled = enabled
	refreshRoutingPolicyCaches()
	return policy, nil
}

// DeleteRoutingPolicyById 删除策略及其目录行；被别名引用时拒绝。
func DeleteRoutingPolicyById(id int) error {
	current, err := GetRoutingPolicyById(id)
	if err != nil {
		return err
	}
	if !current.IsOwnPolicy() {
		refs, err := ModelAliasReferences(current.Name)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return routingPolicyReferenced(refs)
		}
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&RoutingPolicy{}, id).Error; err != nil {
			return err
		}
		if current.IsOwnPolicy() {
			return nil
		}
		return tx.Where("model = ? AND kind = ?", current.Name, ModelInfoKindRoutingPolicy).Delete(&ModelInfo{}).Error
	})
	if err != nil {
		return err
	}
	refreshRoutingPolicyCaches()
	return nil
}

// ---- 进程内缓存 ----
//
// 请求路径按名查策略，不能每次打库。与 modelInfoCache 一样是进程内索引：本进程写入后立即刷新，
// 其它实例在下次启动时加载。

// routingPolicyKey indexes a policy by scope and name.
type routingPolicyKey struct {
	scope RoutingScope
	name  string
}

type routingPolicyIndex struct {
	sync.RWMutex
	loaded bool
	byKey  map[routingPolicyKey]*RoutingPolicy
	// byScope lists each scope's policies for model listings.
	byScope map[RoutingScope][]*RoutingPolicy
}

var routingPolicyCache routingPolicyIndex

// ReloadRoutingPolicyCache 全量重建策略索引。
func ReloadRoutingPolicyCache() error {
	policies, err := GetRoutingPolicies()
	if err != nil {
		return err
	}
	byKey := make(map[routingPolicyKey]*RoutingPolicy, len(policies))
	byScope := make(map[RoutingScope][]*RoutingPolicy)
	for _, policy := range policies {
		byKey[routingPolicyKey{policy.Scope(), policy.Name}] = policy
		byScope[policy.Scope()] = append(byScope[policy.Scope()], policy)
	}
	routingPolicyCache.Lock()
	defer routingPolicyCache.Unlock()
	routingPolicyCache.byKey = byKey
	routingPolicyCache.byScope = byScope
	routingPolicyCache.loaded = true
	return nil
}

// ResetRoutingPolicyCache 清空索引，之后按名查找回落 DB（DB 为 nil 时查不到）。供测试还原。
func ResetRoutingPolicyCache() {
	routingPolicyCache.Lock()
	defer routingPolicyCache.Unlock()
	routingPolicyCache.byKey = nil
	routingPolicyCache.byScope = nil
	routingPolicyCache.loaded = false
}

func refreshRoutingPolicyCaches() {
	routingPolicyCache.RLock()
	loaded := routingPolicyCache.loaded
	routingPolicyCache.RUnlock()
	if loaded {
		if err := ReloadRoutingPolicyCache(); err != nil {
			logger.SysError("Failed to reload routing policy cache: " + err.Error())
		}
	}
	// 目录行随策略增删改，依赖 model_info 的缓存也要跟着刷。
	refreshCatalogCaches()
}

// lookupScopedRoutingPolicy finds a policy of one scope by name (disabled ones included); the index
// falls back to the database when it is not loaded, and returns nil when nothing matches.
func lookupScopedRoutingPolicy(scope RoutingScope, name string) *RoutingPolicy {
	if name == "" {
		return nil
	}
	routingPolicyCache.RLock()
	if routingPolicyCache.loaded {
		policy := routingPolicyCache.byKey[routingPolicyKey{scope, name}]
		routingPolicyCache.RUnlock()
		return policy
	}
	routingPolicyCache.RUnlock()
	if DB == nil {
		return nil
	}
	policy, err := getRoutingPolicyByScopedName(scope, name)
	if err != nil {
		return nil
	}
	return policy
}

// scopedRoutingPolicies lists the policies of one scope (disabled ones included).
func scopedRoutingPolicies(scope RoutingScope) []*RoutingPolicy {
	routingPolicyCache.RLock()
	if routingPolicyCache.loaded {
		policies := append([]*RoutingPolicy(nil), routingPolicyCache.byScope[scope]...)
		routingPolicyCache.RUnlock()
		return policies
	}
	routingPolicyCache.RUnlock()
	if DB == nil {
		return nil
	}
	policies, err := GetRoutingPoliciesByScope(scope)
	if err != nil {
		return nil
	}
	return policies
}

// LookupRoutingPolicy 按名查平台策略（含已禁用的）；索引未加载时回落 DB，查不到返回 nil。
func LookupRoutingPolicy(name string) *RoutingPolicy {
	return lookupScopedRoutingPolicy(PlatformRoutingScope, name)
}

// ownPoliciesActive reports whether the scope's own policies take part in resolution.
func ownPoliciesActive(scope RoutingScope) bool {
	return !scope.IsPlatform() && config.UserRoutingPolicyEnabled
}

// ResolveRoutingPolicy returns the enabled policy a request for the model name uses. scope is the
// API key's own scope (the organization for organization keys, the user for personal keys).
//   - "policy/<name>": only the scope's own policy; nil otherwise, so the name fails as an unknown model
//     (a platform policy is never reached through the prefix, which would skip its catalog row).
//   - "<name>": the scope's own policy (organization policies were called this way before the
//     prefix existed), otherwise the platform policy.
func ResolveRoutingPolicy(scope RoutingScope, requested string) *RoutingPolicy {
	name, prefixed := strings.CutPrefix(requested, RoutingPolicyCallPrefix)
	if ownPoliciesActive(scope) {
		if policy := lookupScopedRoutingPolicy(scope, name); policy != nil && policy.Enabled {
			return policy
		}
	}
	if prefixed {
		return nil
	}
	if policy := LookupRoutingPolicy(name); policy != nil && policy.Enabled {
		return policy
	}
	return nil
}

// IsRoutingPolicyModel 判断模型名是否是一条平台策略（无论启用与否）。
func IsRoutingPolicyModel(name string) bool {
	return LookupRoutingPolicy(name) != nil
}

// EnabledRoutingPolicies 返回所有启用的平台策略，按名排序。
func EnabledRoutingPolicies() []*RoutingPolicy {
	return enabledSorted(scopedRoutingPolicies(PlatformRoutingScope))
}

// EnabledRoutingPoliciesFor returns the enabled policies an API key of the scope can call: its own
// policies plus the platform ones it does not override.
func EnabledRoutingPoliciesFor(scope RoutingScope) []*RoutingPolicy {
	platform := EnabledRoutingPolicies()
	if !ownPoliciesActive(scope) {
		return platform
	}
	own := enabledSorted(scopedRoutingPolicies(scope))
	if len(own) == 0 {
		return platform
	}
	overridden := make(map[string]bool, len(own))
	for _, policy := range own {
		overridden[policy.Name] = true
	}
	merged := own
	for _, policy := range platform {
		if !overridden[policy.Name] {
			merged = append(merged, policy)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].CallName() < merged[j].CallName() })
	return merged
}

func enabledSorted(all []*RoutingPolicy) []*RoutingPolicy {
	enabled := make([]*RoutingPolicy, 0, len(all))
	for _, policy := range all {
		if policy.Enabled {
			enabled = append(enabled, policy)
		}
	}
	sort.Slice(enabled, func(i, j int) bool { return enabled[i].Name < enabled[j].Name })
	return enabled
}

// RoutingPolicyModelsFor 返回应当并入某个模型列表的策略调用名（自建策略为 policy/<name>）：策略启用，
// 且 routable 对它的兜底模型返回 true（兜底是永远可能被转发到的目标，兜底都到不了的策略对这条列表
// 没有意义）。已在列表里的名字不重复加入。scope selects whose own policies are added on top of the
// platform ones.
func RoutingPolicyModelsFor(scope RoutingScope, existing []string, routable func(modelName string) bool) []string {
	present := make(map[string]bool, len(existing))
	for _, name := range existing {
		present[name] = true
	}
	added := make([]string, 0)
	for _, policy := range EnabledRoutingPoliciesFor(scope) {
		name := policy.CallName()
		if present[name] {
			continue
		}
		if routable != nil && !routable(policy.FallbackModel) {
			continue
		}
		added = append(added, name)
	}
	return added
}

// ManagedRoutingPolicy is a platform policy as users see it in their console: what it is called,
// where it routes and whether their own policy of the same name takes its place. The decision
// question stays with the admins.
type ManagedRoutingPolicy struct {
	Name          string         `json:"name"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	DecisionModel string         `json:"decision_model"`
	Routes        []RoutingRoute `json:"routes"`
	FallbackModel string         `json:"fallback_model"`
	Threshold     float64        `json:"threshold"`
	// Overridden the scope has an enabled policy of the same name, which the bare name now reaches.
	Overridden bool `json:"overridden"`
}

// ManagedRoutingPoliciesFor lists the enabled platform policies the scope's API keys can call: listed
// in the catalog and with a fallback model the scope's group can route (the same rule as the model
// list).
func ManagedRoutingPoliciesFor(scope RoutingScope) ([]ManagedRoutingPolicy, error) {
	group, err := scopeRoutingGroup(scope)
	if err != nil {
		return nil, err
	}
	own := map[string]bool{}
	if ownPoliciesActive(scope) {
		for _, policy := range enabledSorted(scopedRoutingPolicies(scope)) {
			own[policy.Name] = true
		}
	}
	managed := make([]ManagedRoutingPolicy, 0)
	for _, policy := range EnabledRoutingPolicies() {
		if !IsCatalogVisible(policy.Name) {
			continue
		}
		canonical, _ := ResolveCanonicalModel(policy.FallbackModel)
		if _, err := ChannelGroup.GetMatchedModelName(group, canonical); err != nil {
			continue
		}
		routes := make([]RoutingRoute, len(policy.Routes))
		copy(routes, policy.Routes)
		managed = append(managed, ManagedRoutingPolicy{
			Name:          policy.Name,
			DisplayName:   policy.DisplayName,
			Description:   policy.Description,
			DecisionModel: policy.DecisionModel,
			Routes:        routes,
			FallbackModel: policy.FallbackModel,
			Threshold:     policy.Threshold,
			Overridden:    own[policy.Name],
		})
	}
	return managed, nil
}
