package model

import (
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// legacyNotifyChannels reads the notification channels of the config file's old `notify:`
// section, in the shape the settings store them now. Email was on unless disabled, with the
// root user's address when no recipient was set; the others were on when their keys were set.
func legacyNotifyChannels(get func(string) string, disabled func(string) bool) []*NotifyChannel {
	var out []*NotifyChannel
	add := func(typ string, values map[string]string) {
		c := &NotifyChannel{Type: typ, Enabled: true}
		if err := c.SetValues(values, nil); err != nil {
			return
		}
		if _, err := c.Notifier(); err != nil {
			return
		}
		out = append(out, c)
	}
	if !disabled("notify.email.disable") {
		add(notify.TypeEmail, map[string]string{"to": get("notify.email.smtp_to")})
	}
	if !disabled("notify.wecom.disable") && get("notify.wecom.webhook") != "" {
		add(notify.TypeWeCom, map[string]string{"webhook": get("notify.wecom.webhook")})
	}
	for _, typ := range []string{notify.TypeDingTalk, notify.TypeLark} {
		prefix := "notify." + typ + "."
		if get(prefix+"token") != "" {
			add(typ, map[string]string{"token": get(prefix + "token"), "secret": get(prefix + "secret"), "keyword": get(prefix + "keyWord")})
		}
	}
	if get("notify.pushdeer.pushkey") != "" {
		add(notify.TypePushDeer, map[string]string{"pushkey": get("notify.pushdeer.pushkey"), "url": get("notify.pushdeer.url")})
	}
	if get("notify.telegram.bot_api_key") != "" {
		add(notify.TypeTelegram, map[string]string{
			"bot_token":  get("notify.telegram.bot_api_key"),
			"chat_id":    get("notify.telegram.chat_id"),
			"http_proxy": get("notify.telegram.http_proxy"),
		})
	}
	return out
}

// migrateNotifyChannels moves the notification channels from the config file into the
// settings (notify_channels), once. A fresh install gets the same default as before: email to
// the root user's address.
func migrateNotifyChannels() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611150101",
		Migrate: func(tx *gorm.DB) error {
			var count int64
			if err := tx.Model(&NotifyChannel{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			channels := legacyNotifyChannels(viper.GetString, viper.GetBool)
			now := utils.GetTimestamp()
			for i, c := range channels {
				c.Sort, c.CreatedTime, c.UpdatedTime = i, now, now
				if err := tx.Create(c).Error; err != nil {
					return err
				}
			}
			logger.SysLog("notification channels moved from the config file into the settings")
			return nil
		},
	}
}

// foldChannelDisableNotifyOption retires the AutomaticDisableChannelNotifyEnabled switch: it
// duplicated the channel_disabled kind of the notification settings, so turning it off now
// turns that kind off.
func foldChannelDisableNotifyOption() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611150102",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("options") {
				return nil
			}
			var options []Option
			if err := tx.Find(&options).Error; err != nil {
				return err
			}
			values := map[string]string{}
			for _, option := range options {
				values[option.Key] = option.Value
			}
			old, exists := values["AutomaticDisableChannelNotifyEnabled"]
			if !exists {
				return nil
			}
			if old == "false" {
				kinds := disabledKindsWith(values["NotifyDisabledKinds"], notify.KindChannelDisabled)
				if err := tx.Save(&Option{Key: "NotifyDisabledKinds", Value: kinds}).Error; err != nil {
					return err
				}
			}
			return tx.Delete(&Option{Key: "AutomaticDisableChannelNotifyEnabled"}).Error
		},
	}
}

// disabledKindsWith adds kind to a comma separated list of disabled kinds.
func disabledKindsWith(list, kind string) string {
	var out []string
	for _, k := range strings.Split(list, ",") {
		if k = strings.TrimSpace(k); k != "" {
			if k == kind {
				return list
			}
			out = append(out, k)
		}
	}
	return strings.Join(append(out, kind), ",")
}
