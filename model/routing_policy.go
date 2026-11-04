package model

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common"
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
	Id          int    `json:"id"`
	Name        string `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_routing_policies_name"`
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
	ErrRoutingPolicyReferenced       = common.NewAPIError(common.APICodeRoutingPolicyReferenced, "policy is referenced by model aliases", nil)
)

func routingPolicyModelRequired(field string) error {
	return common.NewAPIError(common.APICodeRoutingPolicyModelRequired, field+" is required", map[string]any{"field": field})
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

func GetRoutingPolicies() ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := DB.Order("id asc").Find(&policies).Error
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

func GetRoutingPolicyByName(name string) (*RoutingPolicy, error) {
	policy := &RoutingPolicy{}
	err := DB.First(policy, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoutingPolicyNotFound
	}
	return policy, err
}

// routingPolicyNameTaken 判断名字是否已被目录里的模型（含别名）或另一条策略占用。
// 渠道提供的模型未必有目录行，所以还要问 balancer；它没加载时视为未占用。
func routingPolicyNameTaken(name string, selfId int) (bool, error) {
	if info, err := GetModelInfoByModel(name); err == nil && info != nil {
		// 自己联动出来的那一行不算占用（改名以外的更新会走到这里）。
		if info.Kind != ModelInfoKindRoutingPolicy {
			return true, nil
		}
		if other, err := GetRoutingPolicyByName(name); err == nil && other.Id != selfId {
			return true, nil
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if other, err := GetRoutingPolicyByName(name); err == nil && other.Id != selfId {
		return true, nil
	}
	if ChannelGroup.HasModel(name) {
		return true, nil
	}
	return false, nil
}

func isOtherRoutingPolicy(selfId int) func(name string) bool {
	return func(name string) bool {
		other, err := GetRoutingPolicyByName(name)
		return err == nil && other.Id != selfId
	}
}

// ---- 写入 ----

// Insert 校验并写入策略，同一事务里建目录行。
func (p *RoutingPolicy) Insert() error {
	p.Normalize()
	if err := p.Validate(isOtherRoutingPolicy(0)); err != nil {
		return err
	}
	taken, err := routingPolicyNameTaken(p.Name, 0)
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
		return tx.Create(p.catalogRow(nil)).Error
	})
	if err != nil {
		return err
	}
	refreshRoutingPolicyCaches()
	return nil
}

// Update 校验并整行保存策略；改名时目录行跟着改名，被别名引用的旧名不允许改。
func (p *RoutingPolicy) Update() error {
	current, err := GetRoutingPolicyById(p.Id)
	if err != nil {
		return err
	}
	p.Normalize()
	if err := p.Validate(isOtherRoutingPolicy(p.Id)); err != nil {
		return err
	}
	if p.Name != current.Name {
		taken, err := routingPolicyNameTaken(p.Name, p.Id)
		if err != nil {
			return err
		}
		if taken {
			return ErrRoutingPolicyNameTaken
		}
		refs, err := ModelAliasReferences(current.Name)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return routingPolicyReferenced(refs)
		}
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("created_at").Save(p).Error; err != nil {
			return err
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
	refs, err := ModelAliasReferences(current.Name)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return routingPolicyReferenced(refs)
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&RoutingPolicy{}, id).Error; err != nil {
			return err
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

type routingPolicyIndex struct {
	sync.RWMutex
	loaded bool
	byName map[string]*RoutingPolicy
}

var routingPolicyCache routingPolicyIndex

// ReloadRoutingPolicyCache 全量重建策略索引。
func ReloadRoutingPolicyCache() error {
	policies, err := GetRoutingPolicies()
	if err != nil {
		return err
	}
	byName := make(map[string]*RoutingPolicy, len(policies))
	for _, policy := range policies {
		byName[policy.Name] = policy
	}
	routingPolicyCache.Lock()
	defer routingPolicyCache.Unlock()
	routingPolicyCache.byName = byName
	routingPolicyCache.loaded = true
	return nil
}

// ResetRoutingPolicyCache 清空索引，之后按名查找回落 DB（DB 为 nil 时查不到）。供测试还原。
func ResetRoutingPolicyCache() {
	routingPolicyCache.Lock()
	defer routingPolicyCache.Unlock()
	routingPolicyCache.byName = nil
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

// LookupRoutingPolicy 按名查策略（含已禁用的）；索引未加载时回落 DB，查不到返回 nil。
func LookupRoutingPolicy(name string) *RoutingPolicy {
	if name == "" {
		return nil
	}
	routingPolicyCache.RLock()
	if routingPolicyCache.loaded {
		policy := routingPolicyCache.byName[name]
		routingPolicyCache.RUnlock()
		return policy
	}
	routingPolicyCache.RUnlock()
	if DB == nil {
		return nil
	}
	policy, err := GetRoutingPolicyByName(name)
	if err != nil {
		return nil
	}
	return policy
}

// IsRoutingPolicyModel 判断模型名是否是一条策略（无论启用与否）。
func IsRoutingPolicyModel(name string) bool {
	return LookupRoutingPolicy(name) != nil
}

// EnabledRoutingPolicies 返回所有启用的策略，按名排序。
func EnabledRoutingPolicies() []*RoutingPolicy {
	var all []*RoutingPolicy
	routingPolicyCache.RLock()
	if routingPolicyCache.loaded {
		all = make([]*RoutingPolicy, 0, len(routingPolicyCache.byName))
		for _, policy := range routingPolicyCache.byName {
			all = append(all, policy)
		}
		routingPolicyCache.RUnlock()
	} else {
		routingPolicyCache.RUnlock()
		if DB == nil {
			return nil
		}
		var err error
		all, err = GetRoutingPolicies()
		if err != nil {
			return nil
		}
	}
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
// 已在列表里的名字不重复加入。
func RoutingPolicyModelsFor(existing []string, routable func(modelName string) bool) []string {
	present := make(map[string]bool, len(existing))
	for _, name := range existing {
		present[name] = true
	}
	added := make([]string, 0)
	for _, policy := range EnabledRoutingPolicies() {
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
