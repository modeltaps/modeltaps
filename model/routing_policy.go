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
	// OrganizationId is 0 for platform policies (managed by system admins, usable by everyone) and the
	// owning organization otherwise (usable only by that organization's API keys). Names are unique
	// within one scope; an organization policy may reuse a platform policy's name and then wins for
	// that organization.
	OrganizationId int    `json:"organization_id" gorm:"not null;default:0;uniqueIndex:idx_routing_policies_org_name,priority:1"`
	Name           string `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_routing_policies_org_name,priority:2"`
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

// routingPolicyNamePattern 策略名会作为模型名进 URL（Gemini 的 models/<name>）与各处选择器，
// 只放行字母、数字与 . _ / - ，且不能以 / 开头或结尾。
var routingPolicyNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

var (
	ErrRoutingPolicyNotFound         = common.NewAPIError(common.APICodeRoutingPolicyNotFound, "routing policy not found", nil)
	ErrRoutingPolicyNameInvalid      = common.NewAPIError(common.APICodeRoutingPolicyNameInvalid, "name may only contain letters, digits and . _ / -, up to 100 characters, and must not start or end with /", nil)
	ErrRoutingPolicyNameTaken        = common.NewAPIError(common.APICodeRoutingPolicyNameTaken, "name is already used by a model or another policy", nil)
	ErrRoutingPolicyRoutesCount      = common.NewAPIError(common.APICodeRoutingPolicyRoutesCount, "a policy needs between 2 and 16 routes", map[string]any{"min": RoutingPolicyMinRoutes, "max": RoutingPolicyMaxRoutes})
	ErrRoutingPolicyThresholdInvalid = common.NewAPIError(common.APICodeRoutingPolicyThresholdInvalid, "threshold must be between 0 and 1", nil)
	ErrRoutingPolicyScoringInvalid   = common.NewAPIError(common.APICodeRoutingPolicyScoringInvalid, "scoring must be auto, logprobs or self_reported", nil)
	ErrRoutingPolicyReferenced       = common.NewAPIError(common.APICodeRoutingPolicyReferenced, "policy is referenced by model aliases", nil)
	ErrOrgRoutingPolicyDisabled      = common.NewAPIError(common.APICodeOrgRoutingPolicyDisabled, "organization routing policies are not enabled", nil)
)

// routingPolicyModelUnavailable a model an organization policy refers to is hidden or has no
// channel in the organization's group.
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
	if name == "" || len(name) > routingPolicyMaxNameLen || !routingPolicyNamePattern.MatchString(name) || strings.HasSuffix(name, "/") {
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

// IsOrganizationPolicy reports whether the policy belongs to an organization rather than the platform.
func (p *RoutingPolicy) IsOrganizationPolicy() bool {
	return p.OrganizationId > 0
}

// GetRoutingPolicies returns every policy, platform and organization ones alike.
func GetRoutingPolicies() ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Order("id asc").Find(&policies).Error
	return policies, err
}

// GetRoutingPoliciesByOrganization returns the policies of one scope (0 = platform).
func GetRoutingPoliciesByOrganization(organizationId int) ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Where("organization_id = ?", organizationId).Order("id asc").Find(&policies).Error
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

// GetOrganizationRoutingPolicyById returns the policy only when it belongs to the organization, so
// one organization can never read or change another's policy by guessing an id.
func GetOrganizationRoutingPolicyById(organizationId, id int) (*RoutingPolicy, error) {
	policy, err := GetRoutingPolicyById(id)
	if err != nil {
		return nil, err
	}
	if policy.OrganizationId != organizationId {
		return nil, ErrRoutingPolicyNotFound
	}
	return policy, nil
}

// GetRoutingPolicyByName looks up a platform policy by name.
func GetRoutingPolicyByName(name string) (*RoutingPolicy, error) {
	return getRoutingPolicyByScopedName(0, name)
}

func getRoutingPolicyByScopedName(organizationId int, name string) (*RoutingPolicy, error) {
	policy := &RoutingPolicy{}
	err := DB.First(policy, "organization_id = ? AND name = ?", organizationId, name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoutingPolicyNotFound
	}
	return policy, err
}

func countOrganizationRoutingPolicies(organizationId int) (int64, error) {
	var count int64
	err := DB.Model(&RoutingPolicy{}).Where("organization_id = ?", organizationId).Count(&count).Error
	return count, err
}

// routingPolicyNameTaken 判断名字是否已被目录里的模型（含别名）或同一作用域里的另一条策略占用。
// 渠道提供的模型未必有目录行，所以还要问 balancer；它没加载时视为未占用。
// An organization policy may share a platform policy's name (it overrides it for that organization),
// so the catalog row a platform policy owns does not count against it.
func routingPolicyNameTaken(organizationId int, name string, selfId int) (bool, error) {
	if info, err := GetModelInfoByModel(name); err == nil && info != nil {
		if info.Kind != ModelInfoKindRoutingPolicy {
			return true, nil
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if other, err := getRoutingPolicyByScopedName(organizationId, name); err == nil && other.Id != selfId {
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
// could see: platform policies for everyone, plus the organization's own policies. Policies must not
// point at policies, so this is the "no nesting" check of Validate.
func isOtherRoutingPolicy(organizationId, selfId int) func(name string) bool {
	return func(name string) bool {
		if other := LookupRoutingPolicy(name); other != nil && other.Id != selfId {
			return true
		}
		if organizationId > 0 {
			if other := lookupScopedRoutingPolicy(organizationId, name); other != nil && other.Id != selfId {
				return true
			}
		}
		return false
	}
}

// IsRoutingPolicyModelFor is the nesting check for drafts of the given scope (0 = platform).
func IsRoutingPolicyModelFor(organizationId int) func(name string) bool {
	return isOtherRoutingPolicy(organizationId, 0)
}

// ValidateOrganizationModels checks that every model an organization policy refers to can be used
// by the organization: it is not hidden in the catalog and the organization's group has a channel
// for it. Platform admins hide models they only want reachable through their own policies; an
// organization policy must not reach them.
func (p *RoutingPolicy) ValidateOrganizationModels(organizationId int) error {
	group, err := organizationRoutingGroup(organizationId)
	if err != nil {
		return err
	}
	check := func(field, name string) error {
		return checkOrganizationRoutingModel(group, field, name)
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

// CheckOrganizationRoutingModel is ValidateOrganizationModels for a single model (used by the
// decision model probe of the organization console).
func CheckOrganizationRoutingModel(organizationId int, field, name string) error {
	group, err := organizationRoutingGroup(organizationId)
	if err != nil {
		return err
	}
	return checkOrganizationRoutingModel(group, field, name)
}

func checkOrganizationRoutingModel(group, field, name string) error {
	if !OrganizationRoutingTargetUsable(name) {
		return routingPolicyModelUnavailable(field, name)
	}
	canonical, _ := ResolveCanonicalModel(name)
	if _, err := ChannelGroup.GetMatchedModelName(group, canonical); err != nil {
		return routingPolicyModelUnavailable(field, name)
	}
	return nil
}

// OrganizationRoutingModels lists the models an organization policy may use, for the console's
// model pickers: routable in the organization's group, visible in the catalog, not an alias and not
// a policy. Sorted by name.
func OrganizationRoutingModels(organizationId int) ([]string, error) {
	group, err := organizationRoutingGroup(organizationId)
	if err != nil {
		return nil, err
	}
	models, err := ChannelGroup.GetGroupModels(group)
	if err != nil {
		return []string{}, nil
	}
	// Targets stay inside the platform's approved models of the organization (ORG-15).
	var approved []string
	var org Organization
	if err := DB.Select("id", "approved_models").Where("id = ?", organizationId).First(&org).Error; err == nil {
		approved = []string(org.ApprovedModels)
	}
	isPolicy := IsRoutingPolicyModelFor(organizationId)
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

// OrganizationRoutingTargetUsable reports whether an organization policy may send a request to the
// model: hidden catalog models are off limits (checked again at request time, since the platform
// may hide a model after the policy was saved).
func OrganizationRoutingTargetUsable(name string) bool {
	canonical, _ := ResolveCanonicalModel(name)
	return !(config.CatalogEnforceHidden() && IsModelHidden(canonical))
}

// organizationRoutingGroup returns the user group of the organization's shadow account, which is the
// group its API keys route in by default.
func organizationRoutingGroup(organizationId int) (string, error) {
	org, err := GetOrganizationById(organizationId)
	if err != nil {
		return "", err
	}
	return CacheGetUserGroup(org.ShadowUserId)
}

// ---- 写入 ----

// validateForWrite runs the checks shared by Insert and Update.
func (p *RoutingPolicy) validateForWrite() error {
	p.Normalize()
	if err := p.Validate(isOtherRoutingPolicy(p.OrganizationId, p.Id)); err != nil {
		return err
	}
	if p.IsOrganizationPolicy() {
		return p.ValidateOrganizationModels(p.OrganizationId)
	}
	return nil
}

// Insert 校验并写入策略；平台策略在同一事务里建目录行。Organization policies get no catalog row:
// the catalog is global and would show the policy to every other organization.
func (p *RoutingPolicy) Insert() error {
	if err := p.validateForWrite(); err != nil {
		return err
	}
	if p.IsOrganizationPolicy() {
		count, err := countOrganizationRoutingPolicies(p.OrganizationId)
		if err != nil {
			return err
		}
		if max := config.OrganizationRoutingPolicyMaxCount; max > 0 && count >= int64(max) {
			return routingPolicyCountExceeded(max)
		}
	}
	taken, err := routingPolicyNameTaken(p.OrganizationId, p.Name, 0)
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
		if p.IsOrganizationPolicy() {
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
// The owning scope never changes: a policy cannot move between the platform and an organization.
func (p *RoutingPolicy) Update() error {
	current, err := GetRoutingPolicyById(p.Id)
	if err != nil {
		return err
	}
	p.OrganizationId = current.OrganizationId
	if err := p.validateForWrite(); err != nil {
		return err
	}
	if p.Name != current.Name {
		taken, err := routingPolicyNameTaken(p.OrganizationId, p.Name, p.Id)
		if err != nil {
			return err
		}
		if taken {
			return ErrRoutingPolicyNameTaken
		}
		if !p.IsOrganizationPolicy() {
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
		if p.IsOrganizationPolicy() {
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
	if !current.IsOrganizationPolicy() {
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
		if current.IsOrganizationPolicy() {
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

// routingPolicyKey indexes a policy by scope (0 = platform) and name.
type routingPolicyKey struct {
	organizationId int
	name           string
}

type routingPolicyIndex struct {
	sync.RWMutex
	loaded bool
	byKey  map[routingPolicyKey]*RoutingPolicy
	// byOrganization lists each organization's policies for model listings.
	byOrganization map[int][]*RoutingPolicy
}

var routingPolicyCache routingPolicyIndex

// ReloadRoutingPolicyCache 全量重建策略索引。
func ReloadRoutingPolicyCache() error {
	policies, err := GetRoutingPolicies()
	if err != nil {
		return err
	}
	byKey := make(map[routingPolicyKey]*RoutingPolicy, len(policies))
	byOrganization := make(map[int][]*RoutingPolicy)
	for _, policy := range policies {
		byKey[routingPolicyKey{policy.OrganizationId, policy.Name}] = policy
		byOrganization[policy.OrganizationId] = append(byOrganization[policy.OrganizationId], policy)
	}
	routingPolicyCache.Lock()
	defer routingPolicyCache.Unlock()
	routingPolicyCache.byKey = byKey
	routingPolicyCache.byOrganization = byOrganization
	routingPolicyCache.loaded = true
	return nil
}

// ResetRoutingPolicyCache 清空索引，之后按名查找回落 DB（DB 为 nil 时查不到）。供测试还原。
func ResetRoutingPolicyCache() {
	routingPolicyCache.Lock()
	defer routingPolicyCache.Unlock()
	routingPolicyCache.byKey = nil
	routingPolicyCache.byOrganization = nil
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
func lookupScopedRoutingPolicy(organizationId int, name string) *RoutingPolicy {
	if name == "" {
		return nil
	}
	routingPolicyCache.RLock()
	if routingPolicyCache.loaded {
		policy := routingPolicyCache.byKey[routingPolicyKey{organizationId, name}]
		routingPolicyCache.RUnlock()
		return policy
	}
	routingPolicyCache.RUnlock()
	if DB == nil {
		return nil
	}
	policy, err := getRoutingPolicyByScopedName(organizationId, name)
	if err != nil {
		return nil
	}
	return policy
}

// scopedRoutingPolicies lists the policies of one scope (disabled ones included).
func scopedRoutingPolicies(organizationId int) []*RoutingPolicy {
	routingPolicyCache.RLock()
	if routingPolicyCache.loaded {
		policies := append([]*RoutingPolicy(nil), routingPolicyCache.byOrganization[organizationId]...)
		routingPolicyCache.RUnlock()
		return policies
	}
	routingPolicyCache.RUnlock()
	if DB == nil {
		return nil
	}
	policies, err := GetRoutingPoliciesByOrganization(organizationId)
	if err != nil {
		return nil
	}
	return policies
}

// LookupRoutingPolicy 按名查平台策略（含已禁用的）；索引未加载时回落 DB，查不到返回 nil。
func LookupRoutingPolicy(name string) *RoutingPolicy {
	return lookupScopedRoutingPolicy(0, name)
}

// organizationPoliciesActive reports whether organization policies take part in resolution.
func organizationPoliciesActive(organizationId int) bool {
	return organizationId > 0 && config.OrganizationRoutingPolicyEnabled
}

// ResolveRoutingPolicy returns the enabled policy a request for name uses: the organization's own
// policy when it has an enabled one with that name, otherwise the enabled platform policy, otherwise
// nil. organizationId is 0 for keys outside any organization.
func ResolveRoutingPolicy(organizationId int, name string) *RoutingPolicy {
	if organizationPoliciesActive(organizationId) {
		if policy := lookupScopedRoutingPolicy(organizationId, name); policy != nil && policy.Enabled {
			return policy
		}
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
	return enabledSorted(scopedRoutingPolicies(0))
}

// EnabledRoutingPoliciesFor returns the enabled policies an API key of the organization can call
// (0 = outside any organization): its own policies plus the platform ones it does not override.
func EnabledRoutingPoliciesFor(organizationId int) []*RoutingPolicy {
	platform := EnabledRoutingPolicies()
	if !organizationPoliciesActive(organizationId) {
		return platform
	}
	own := enabledSorted(scopedRoutingPolicies(organizationId))
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
	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })
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

// RoutingPolicyModelsFor 返回应当并入某个模型列表的策略名：策略启用，且 routable 对它的兜底
// 模型返回 true（兜底是永远可能被转发到的目标，兜底都到不了的策略对这条列表没有意义）。
// 已在列表里的名字不重复加入。organizationId selects which organization's own policies are added
// on top of the platform ones (0 = none).
func RoutingPolicyModelsFor(organizationId int, existing []string, routable func(modelName string) bool) []string {
	present := make(map[string]bool, len(existing))
	for _, name := range existing {
		present[name] = true
	}
	added := make([]string, 0)
	for _, policy := range EnabledRoutingPoliciesFor(organizationId) {
		if present[policy.Name] {
			continue
		}
		if routable != nil && !routable(policy.FallbackModel) {
			continue
		}
		added = append(added, policy.Name)
	}
	return added
}
