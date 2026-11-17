package model

import (
	"strconv"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"gorm.io/gorm"
)

// migrateMailServices turns the mail settings saved before mail services existed (the SMTP*
// options) into the first mail service, in use. The options stay: they now mirror it.
func migrateMailServices() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611170401",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("options") {
				return nil
			}
			var count int64
			if err := tx.Model(&MailService{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			var options []Option
			if err := tx.Find(&options).Error; err != nil {
				return err
			}
			values := map[string]string{}
			for _, o := range options {
				values[o.Key] = o.Value
			}
			if values["SMTPServer"] == "" {
				return nil
			}
			port, err := strconv.Atoi(values["SMTPPort"])
			if err != nil || port <= 0 {
				port = 587
			}
			now := utils.GetTimestamp()
			s := &MailService{
				Provider: values["SMTPProvider"], Server: values["SMTPServer"], Port: port,
				Account: values["SMTPAccount"], Token: values["SMTPToken"], TLSMode: values["SMTPTLSMode"],
				From: values["SMTPFrom"], FromName: values["SMTPFromName"], ReplyTo: values["SMTPReplyTo"],
				Active: true, CreatedTime: now, UpdatedTime: now,
			}
			s.Normalize()
			if err := tx.Create(s).Error; err != nil {
				return err
			}
			logger.SysLog("mail settings moved into the first mail service")
			return nil
		},
	}
}
