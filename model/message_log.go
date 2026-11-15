package model

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/common/utils"
)

// MessageLog is one email or admin notice the platform tried to send, kept so the console can show
// what went out and why something did not arrive. Source is "mail" for every email (user emails,
// tests and email notices alike) and "notify" for every notification channel delivery.
type MessageLog struct {
	Id        int    `json:"id"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
	Source    string `json:"source" gorm:"type:varchar(16);index"`
	Channel   string `json:"channel" gorm:"type:varchar(16)"`
	Target    string `json:"target" gorm:"type:varchar(255)"`
	Kind      string `json:"kind" gorm:"type:varchar(32)"`
	Title     string `json:"title" gorm:"type:varchar(255)"`
	Success   bool   `json:"success"`
	Error     string `json:"error" gorm:"type:text"`
}

const (
	MessageSourceMail   = "mail"
	MessageSourceNotify = "notify"

	// messageLogRetention is how long records are kept.
	messageLogRetention = 30 * 24 * time.Hour
	// messageLogPruneEvery prunes old records once per this many inserts.
	messageLogPruneEvery = 200
)

var messageLogInserts atomic.Int64

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// RecordMessage stores a send record without holding up the sender.
func RecordMessage(entry MessageLog) {
	if DB == nil {
		return
	}
	entry.CreatedAt = utils.GetTimestamp()
	entry.Target, entry.Title = clip(entry.Target, 255), clip(entry.Title, 255)
	entry.Kind, entry.Channel = clip(entry.Kind, 32), clip(entry.Channel, 16)
	entry.Error = clip(entry.Error, 2000)
	go func() {
		if err := DB.Create(&entry).Error; err != nil {
			logger.SysError("save message log failed: " + err.Error())
			return
		}
		if messageLogInserts.Add(1)%messageLogPruneEvery == 0 {
			PruneMessageLogs()
		}
	}()
}

// PruneMessageLogs drops records older than the retention period.
func PruneMessageLogs() {
	cutoff := time.Now().Add(-messageLogRetention).Unix()
	if err := DB.Where("created_at < ?", cutoff).Delete(&MessageLog{}).Error; err != nil {
		logger.SysError("prune message logs failed: " + err.Error())
	}
}

// GetMessageLogs lists the newest records of a source, with the total count. failedOnly keeps only
// the ones that did not go out.
func GetMessageLogs(source string, failedOnly bool, offset, limit int) ([]*MessageLog, int64, error) {
	q := DB.Model(&MessageLog{}).Where("source = ?", source)
	if failedOnly {
		q = q.Where("success = ?", false)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []*MessageLog
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// RecordSentMail is the stmp.OnSent hook.
func RecordSentMail(r stmp.SendRecord) {
	RecordMessage(MessageLog{
		Source: MessageSourceMail, Channel: "email", Target: r.To, Kind: r.Kind, Title: r.Subject,
		Success: r.Err == nil, Error: errorText(r.Err),
	})
}

// RecordNotice is the notify.OnDelivered hook.
func RecordNotice(d notify.Delivery) {
	target := d.Name
	if target == "" {
		target = d.Type
	}
	RecordMessage(MessageLog{
		Source: MessageSourceNotify, Channel: d.Type, Target: target, Kind: d.Kind, Title: d.Title,
		Success: d.Err == nil, Error: errorText(d.Err),
	})
}
