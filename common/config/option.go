package config

import (
	"strconv"
	"sync"
)

// OptionHandler 定义配置项处理器接口
type OptionHandler interface {
	// SetValue 设置配置值
	SetValue(value string) error
	// GetValue 获取配置字符串值
	GetValue() string
}

// OptionScope 声明配置项的下发范围
type OptionScope int

const (
	// ScopeAdmin 仅通过 /api/option/(RootAuth)下发
	ScopeAdmin OptionScope = iota
	// ScopePublic 除 /api/option/ 外,还通过 /api/status 匿名下发
	ScopePublic
	// ScopeSecret 永不下发给任何客户端
	ScopeSecret
)

// PublicSpec 描述 public 配置项在 /api/status 响应中的呈现方式
type PublicSpec struct {
	// StatusKey /api/status 响应中的字段名,为空时使用注册 key
	StatusKey string
	// StatusValue 返回 /api/status 中的取值(保留原始类型),为空时使用注册时的类型化取值
	StatusValue func() any
}

type optionEntry struct {
	handler     OptionHandler
	scope       OptionScope
	statusKey   string
	statusValue func() any
}

// OptionManager 配置管理器
type OptionManager struct {
	entries map[string]*optionEntry
	mutex   *sync.RWMutex
}

var GlobalOption = NewOptionManager()

// NewOptionManager 创建配置管理器实例
func NewOptionManager() *OptionManager {
	return &OptionManager{
		entries: make(map[string]*optionEntry),
		mutex:   &sync.RWMutex{},
	}
}

// Register 注册配置项
func (cm *OptionManager) Register(key string, handler OptionHandler, defaultValue string, scope OptionScope, public ...PublicSpec) {
	cm.register(key, handler, defaultValue, scope, func() any { return handler.GetValue() }, public...)
}

func (cm *OptionManager) register(key string, handler OptionHandler, defaultValue string, scope OptionScope, typedValue func() any, public ...PublicSpec) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	entry := &optionEntry{handler: handler, scope: scope}
	if scope == ScopePublic {
		entry.statusKey = key
		entry.statusValue = typedValue
		if len(public) > 0 {
			if public[0].StatusKey != "" {
				entry.statusKey = public[0].StatusKey
			}
			if public[0].StatusValue != nil {
				entry.statusValue = public[0].StatusValue
			}
		}
	}
	cm.entries[key] = entry

	// 设置默认值
	if defaultValue != "" {
		handler.SetValue(defaultValue)
	}
}

// RegisterString 快速注册字符串配置
func (cm *OptionManager) RegisterString(key string, value *string, scope OptionScope, public ...PublicSpec) {
	cm.register(key, &StringOptionHandler{
		value: value,
	}, "", scope, func() any { return *value }, public...)
}

// RegisterBool 快速注册布尔配置
func (cm *OptionManager) RegisterBool(key string, value *bool, scope OptionScope, public ...PublicSpec) {
	cm.register(key, &BoolOptionHandler{
		value: value,
	}, "", scope, func() any { return *value }, public...)
}

// RegisterInt 快速注册整数配置
func (cm *OptionManager) RegisterInt(key string, value *int, scope OptionScope, public ...PublicSpec) {
	cm.register(key, &IntOptionHandler{
		value: value,
	}, "", scope, func() any { return *value }, public...)
}

// RegisterFloat 快速注册浮点数配置
func (cm *OptionManager) RegisterFloat(key string, value *float64, scope OptionScope, public ...PublicSpec) {
	cm.register(key, &FloatOptionHandler{
		value: value,
	}, "", scope, func() any { return *value }, public...)
}

// RegisterCustom 注册自定义处理函数的配置
func (cm *OptionManager) RegisterCustom(key string, getter func() string, setter func(string) error, defaultValue string, scope OptionScope, public ...PublicSpec) {
	cm.register(key, &CustomOptionHandler{
		getter: getter,
		setter: setter,
	}, defaultValue, scope, func() any { return getter() }, public...)
}

// RegisterValue 注册一个值类型的配置项
func (cm *OptionManager) RegisterValue(key string, scope OptionScope, public ...PublicSpec) {
	handler := &ValueOptionHandler{
		value: "",
	}
	cm.register(key, handler, "", scope, func() any { return handler.GetValue() }, public...)
}

// Get 获取配置值(字符串)
func (cm *OptionManager) Get(key string) string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	entry, exists := cm.entries[key]
	if !exists {
		return ""
	}
	return entry.handler.GetValue()
}

// Set 设置配置值
func (cm *OptionManager) Set(key string, value string) error {
	handler, exists := cm.getHandler(key)
	if !exists {
		return nil
	}

	return handler.SetValue(value)
}

// 获取处理器
func (cm *OptionManager) getHandler(key string) (OptionHandler, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	entry, exists := cm.entries[key]
	if !exists {
		return nil, false
	}
	return entry.handler, true
}

func (cm *OptionManager) GetAll() map[string]string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	all := make(map[string]string)
	for k, v := range cm.entries {
		all[k] = v.handler.GetValue()
	}
	return all
}

// GetAllNonSecret 返回除 ScopeSecret 外的全部配置(供 /api/option/ 下发)
func (cm *OptionManager) GetAllNonSecret() map[string]string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	all := make(map[string]string)
	for k, v := range cm.entries {
		if v.scope == ScopeSecret {
			continue
		}
		all[k] = v.handler.GetValue()
	}
	return all
}

// PublicOptions 返回 ScopePublic 配置的 statusKey→类型化取值(供 /api/status 下发)
func (cm *OptionManager) PublicOptions() map[string]any {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	public := make(map[string]any)
	for _, v := range cm.entries {
		if v.scope != ScopePublic {
			continue
		}
		public[v.statusKey] = v.statusValue()
	}
	return public
}

// 以下是各种配置处理器的实现
type StringOptionHandler struct {
	value *string
}

func (h *StringOptionHandler) SetValue(value string) error {
	*h.value = value
	return nil
}

func (h *StringOptionHandler) GetValue() string {
	return *h.value
}

type BoolOptionHandler struct {
	value *bool
}

func (h *BoolOptionHandler) SetValue(value string) error {
	*h.value = value == "true"
	return nil
}

func (h *BoolOptionHandler) GetValue() string {
	if *h.value {
		return "true"
	}
	return "false"
}

type IntOptionHandler struct {
	value *int
}

func (h *IntOptionHandler) SetValue(value string) error {
	val, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*h.value = val
	return nil
}

func (h *IntOptionHandler) GetValue() string {
	return strconv.Itoa(*h.value)
}

type FloatOptionHandler struct {
	value *float64
}

func (h *FloatOptionHandler) SetValue(value string) error {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	*h.value = val
	return nil
}

func (h *FloatOptionHandler) GetValue() string {
	return strconv.FormatFloat(*h.value, 'f', -1, 64)
}

type CustomOptionHandler struct {
	getter func() string
	setter func(string) error
}

func (h *CustomOptionHandler) SetValue(value string) error {
	return h.setter(value)
}

func (h *CustomOptionHandler) GetValue() string {
	return h.getter()
}

// ValueOptionHandler 用于存储非全局变量的字符串值
type ValueOptionHandler struct {
	value string
}

func (h *ValueOptionHandler) SetValue(value string) error {
	h.value = value
	return nil
}

func (h *ValueOptionHandler) GetValue() string {
	return h.value
}
