package model

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupMailServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&MailService{}, &Option{}); err != nil {
		t.Fatal(err)
	}
	registerOptions()
	oldDB := DB
	DB = testDB
	saved := []string{config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPFrom}
	t.Cleanup(func() {
		DB = oldDB
		config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPFrom = saved[0], saved[1], saved[2], saved[3]
	})
	return testDB
}

func newTestMailService(server string) *MailService {
	s := &MailService{Server: server, Port: 465, Account: "user@" + server, Token: "secret-" + server, TLSMode: "ssl", From: "noreply@" + server}
	s.Normalize()
	return s
}

func activeMailServiceID(t *testing.T) int {
	t.Helper()
	var rows []MailService
	if err := DB.Where("active = ?", true).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) > 1 {
		t.Fatalf("%d services in use, want at most one", len(rows))
	}
	if len(rows) == 0 {
		return 0
	}
	return rows[0].Id
}

// The first service goes into use and is mirrored into the SMTP options; later ones wait until
// they are switched to, and switching moves the mirror.
func TestMailServiceFirstInUseAndSwitch(t *testing.T) {
	setupMailServiceTestDB(t)
	a, b := newTestMailService("a.example.com"), newTestMailService("b.example.com")
	if err := a.Insert(); err != nil {
		t.Fatal(err)
	}
	if err := b.Insert(); err != nil {
		t.Fatal(err)
	}
	if !a.Active || b.Active || activeMailServiceID(t) != a.Id {
		t.Fatalf("active a=%v b=%v, want only the first", a.Active, b.Active)
	}
	if config.SMTPServer != "a.example.com" || config.SMTPToken != "secret-a.example.com" {
		t.Fatalf("mirror = %s / %s, want the first service", config.SMTPServer, config.SMTPToken)
	}

	// Editing a service not in use does not touch sending.
	b.FromName = "B"
	if err := b.Update(); err != nil {
		t.Fatal(err)
	}
	if config.SMTPServer != "a.example.com" {
		t.Fatalf("editing an unused service changed the mirror to %s", config.SMTPServer)
	}

	if _, err := ActivateMailService(b.Id); err != nil {
		t.Fatal(err)
	}
	if activeMailServiceID(t) != b.Id || config.SMTPServer != "b.example.com" || config.SMTPFromName != "B" {
		t.Fatalf("after switching: active #%d, mirror %s %q", activeMailServiceID(t), config.SMTPServer, config.SMTPFromName)
	}
	var opt Option
	if err := DB.First(&opt, "key = ?", "SMTPServer").Error; err != nil || opt.Value != "b.example.com" {
		t.Fatalf("stored SMTPServer = %q (%v), want the switched-to service", opt.Value, err)
	}
}

// The service in use cannot be deleted while another exists; the last one can, and then
// sending stops.
func TestMailServiceDeleteRules(t *testing.T) {
	setupMailServiceTestDB(t)
	a, b := newTestMailService("a.example.com"), newTestMailService("b.example.com")
	for _, s := range []*MailService{a, b} {
		if err := s.Insert(); err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteMailService(a.Id); !errors.Is(err, ErrMailServiceInUse) {
		t.Fatalf("deleting the service in use = %v, want ErrMailServiceInUse", err)
	}
	if err := DeleteMailService(b.Id); err != nil {
		t.Fatal(err)
	}
	if err := DeleteMailService(a.Id); err != nil {
		t.Fatal(err)
	}
	if config.SMTPServer != "" || config.SMTPToken != "" {
		t.Fatalf("mirror after deleting the last service = %s / %s, want empty", config.SMTPServer, config.SMTPToken)
	}
}

func TestMailServiceValidate(t *testing.T) {
	s := newTestMailService("a.example.com")
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*MailService){
		"no token":   func(s *MailService) { s.Token = "" },
		"bad port":   func(s *MailService) { s.Port = 70000 },
		"bad tls":    func(s *MailService) { s.TLSMode = "tls9" },
		"bad from":   func(s *MailService) { s.From = "not an address" },
		"bad reply":  func(s *MailService) { s.ReplyTo = "nope" },
		"no account": func(s *MailService) { s.Account = "" },
	} {
		c := *s
		change(&c)
		if c.Validate() == nil {
			t.Errorf("%s: Validate passed", name)
		}
	}
}

// Settings saved before mail services existed become the first service, in use.
func TestMigrateMailServicesFromOptions(t *testing.T) {
	db := setupMailServiceTestDB(t)
	for k, v := range map[string]string{
		"SMTPProvider": "resend", "SMTPServer": "smtp.resend.com", "SMTPPort": "465", "SMTPAccount": "resend",
		"SMTPToken": "re_123", "SMTPTLSMode": "ssl", "SMTPFrom": "noreply@example.com", "SMTPFromName": "Example",
	} {
		if err := db.Create(&Option{Key: k, Value: v}).Error; err != nil {
			t.Fatal(err)
		}
	}
	m := migrateMailServices()
	if err := m.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(db); err != nil { // a second run adds nothing
		t.Fatal(err)
	}
	services, err := GetMailServices()
	if err != nil || len(services) != 1 {
		t.Fatalf("services = %d (%v), want 1", len(services), err)
	}
	s := services[0]
	if !s.Active || s.Provider != "resend" || s.Port != 465 || s.Token != "re_123" || s.FromName != "Example" {
		t.Fatalf("migrated service = %+v", s)
	}
}

func TestMigrateMailServicesSkipsWithoutSettings(t *testing.T) {
	db := setupMailServiceTestDB(t)
	if err := migrateMailServices().Migrate(db); err != nil {
		t.Fatal(err)
	}
	services, _ := GetMailServices()
	if len(services) != 0 {
		t.Fatalf("services = %d, want none", len(services))
	}
}
