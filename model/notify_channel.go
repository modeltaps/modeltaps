package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
)

// NotifyChannel is one place admin notices go (email, a WeCom / DingTalk / Lark group bot,
// PushDeer, a Telegram chat). Every enabled channel gets every notice that passes the kind
// switches and the hourly limit (see common/notify).
//
// Settings holds the plain settings and Secrets the secret ones (webhooks, tokens), both as JSON
// objects keyed by the type's field keys (notify.Types). Secrets never leaves the server.
type NotifyChannel struct {
	Id          int    `json:"id"`
	Name        string `json:"name" gorm:"type:varchar(64);default:''"`
	Type        string `json:"type" gorm:"type:varchar(16);not null"`
	Settings    string `json:"-" gorm:"type:text"`
	Secrets     string `json:"-" gorm:"type:text"`
	Enabled     bool   `json:"enabled" gorm:"default:false"`
	Sort        int    `json:"sort" gorm:"default:0"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func decodeValues(raw string) map[string]string {
	values := map[string]string{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &values)
	}
	return values
}

func encodeValues(values map[string]string) string {
	clean := map[string]string{}
	for k, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			clean[k] = v
		}
	}
	data, _ := json.Marshal(clean)
	return string(data)
}

// PlainSettings returns the channel's plain settings.
func (c *NotifyChannel) PlainSettings() map[string]string { return decodeValues(c.Settings) }

// SecretSettings returns the channel's secret settings.
func (c *NotifyChannel) SecretSettings() map[string]string { return decodeValues(c.Secrets) }

// Values returns all settings of the channel, plain and secret.
func (c *NotifyChannel) Values() map[string]string {
	values := c.PlainSettings()
	for k, v := range c.SecretSettings() {
		values[k] = v
	}
	return values
}

// SetValues splits values into the plain and the secret settings of the channel's type and
// drops keys the type does not have. A secret left empty keeps the stored one, since the
// console never gets the stored value back to send it again; the keys in clear are emptied.
func (c *NotifyChannel) SetValues(values map[string]string, clear []string) error {
	fields := notify.FieldsOf(c.Type)
	if fields == nil {
		return fmt.Errorf("unknown notification channel type %q", c.Type)
	}
	stored := c.SecretSettings()
	for _, key := range clear {
		delete(stored, key)
	}
	plain, secret := map[string]string{}, map[string]string{}
	for _, f := range fields {
		v := strings.TrimSpace(values[f.Key])
		if f.Secret {
			if v == "" {
				v = stored[f.Key]
			}
			secret[f.Key] = v
		} else {
			plain[f.Key] = v
		}
	}
	c.Settings, c.Secrets = encodeValues(plain), encodeValues(secret)
	return nil
}

// Notifier builds the channel's notifier, checking its settings.
func (c *NotifyChannel) Notifier() (notify.Notifier, error) {
	return notify.Build(c.Type, c.Values())
}

func GetNotifyChannels() ([]*NotifyChannel, error) {
	var channels []*NotifyChannel
	err := DB.Order("sort asc, id asc").Find(&channels).Error
	return channels, err
}

func GetNotifyChannelById(id int) (*NotifyChannel, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var c NotifyChannel
	if err := DB.First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *NotifyChannel) Insert() error {
	now := utils.GetTimestamp()
	c.CreatedTime, c.UpdatedTime = now, now
	return DB.Create(c).Error
}

func (c *NotifyChannel) Update() error {
	c.UpdatedTime = utils.GetTimestamp()
	return DB.Model(c).Select("name", "settings", "secrets", "enabled", "sort", "updated_time").Updates(c).Error
}

func (c *NotifyChannel) UpdateEnabled(enabled bool) error {
	c.Enabled, c.UpdatedTime = enabled, utils.GetTimestamp()
	return DB.Model(c).Select("enabled", "updated_time").Updates(c).Error
}

func DeleteNotifyChannelById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	return DB.Delete(&NotifyChannel{}, "id = ?", id).Error
}

// NotifyDestinations lists the enabled channels as notifiers. A channel whose settings no longer
// build is skipped and logged; the console shows it as not ready.
func NotifyDestinations() []notify.Destination {
	if DB == nil {
		return nil
	}
	var channels []*NotifyChannel
	if err := DB.Where("enabled = ?", true).Order("sort asc, id asc").Find(&channels).Error; err != nil {
		logger.SysError("load notification channels failed: " + err.Error())
		return nil
	}
	out := make([]notify.Destination, 0, len(channels))
	for _, c := range channels {
		n, err := c.Notifier()
		if err != nil {
			logger.SysError(fmt.Sprintf("notification channel #%d skipped: %s", c.Id, err.Error()))
			continue
		}
		out = append(out, notify.Destination{Name: c.Name, Type: c.Type, Notifier: n})
	}
	return out
}
