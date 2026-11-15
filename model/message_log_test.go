package model

import (
	"errors"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/stmp"
)

func TestMessageLogRecordAndList(t *testing.T) {
	db := setupNotifyTestDB(t)
	// Records are written from another goroutine; an in-memory SQLite is per connection.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&MessageLog{}); err != nil {
		t.Fatal(err)
	}
	RecordSentMail(stmp.SendRecord{Kind: "verification", To: "a@example.com", Subject: "Code"})
	RecordSentMail(stmp.SendRecord{Kind: "test", To: "b@example.com", Subject: "Test", Err: errors.New("535 auth")})
	RecordNotice(notify.Delivery{Kind: notify.KindLowBalance, Type: notify.TypeSlack, Title: "Low"})

	// Records are written in the background.
	deadline := time.Now().Add(2 * time.Second)
	for {
		var n int64
		db.Model(&MessageLog{}).Count(&n)
		if n == 3 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mail, total, err := GetMessageLogs(MessageSourceMail, false, 0, 10)
	if err != nil || total != 2 || len(mail) != 2 || mail[0].Id < mail[1].Id {
		t.Fatalf("mail logs = %v, %d, %v", mail, total, err)
	}
	failed, total, _ := GetMessageLogs(MessageSourceMail, true, 0, 10)
	if total != 1 || failed[0].Error != "535 auth" || failed[0].Success {
		t.Fatalf("failed logs = %v", failed)
	}
	notices, _, _ := GetMessageLogs(MessageSourceNotify, false, 0, 10)
	if len(notices) != 1 || notices[0].Target != notify.TypeSlack || !notices[0].Success {
		t.Fatalf("notify logs = %v", notices)
	}
}

func TestNotifyChannelKinds(t *testing.T) {
	c := &NotifyChannel{}
	c.SetKinds([]string{notify.KindPriceChange, "bogus", notify.KindChannelDisabled})
	if c.Kinds != notify.KindChannelDisabled+","+notify.KindPriceChange {
		t.Fatalf("kinds = %q", c.Kinds)
	}
	if got := c.KindList(); len(got) != 2 {
		t.Fatalf("list = %v", got)
	}
	c.SetKinds(nil)
	if c.Kinds != "" || c.KindList() != nil {
		t.Fatal("empty kinds")
	}
}
