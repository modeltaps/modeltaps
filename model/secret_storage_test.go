package model

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/encryption"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func secretTestKey(fill byte) string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill + byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func useEncryptionKey(t *testing.T, current, previous string) {
	t.Helper()
	if err := encryption.Configure(current, previous); err != nil {
		t.Fatalf("configure encryption: %v", err)
	}
	t.Cleanup(func() { _ = encryption.Configure("", "") })
}

func setupSecretStorageDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := testDB.AutoMigrate(&Channel{}, &Payment{}, &NotifyChannel{}, &OidcProvider{}, &MailService{}, &Option{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	cache.InitCacheManager() // UpdateChannelKey clears the channel's token cache
}

// rawColumn reads a column as stored, past the serializer and the hooks.
func rawColumn(t *testing.T, table, column, where string, args ...any) string {
	t.Helper()
	var stored *string
	row := DB.Raw("SELECT "+quotePostgresField(column)+" FROM "+table+" WHERE "+where, args...).Row()
	if err := row.Scan(&stored); err != nil {
		t.Fatalf("read %s.%s: %v", table, column, err)
	}
	if stored == nil {
		return "<nil>"
	}
	return *stored
}

func assertCiphertext(t *testing.T, stored, plaintext string) {
	t.Helper()
	if !encryption.IsEncrypted(stored) || strings.Contains(stored, plaintext) {
		t.Errorf("stored value is not ciphertext: %q", stored)
	}
}

// secretModels holds every model with a column stored through the encrypted serializer.
var secretModels = map[string]any{
	"Channel":       &Channel{},
	"Payment":       &Payment{},
	"NotifyChannel": &NotifyChannel{},
	"OidcProvider":  &OidcProvider{},
	"MailService":   &MailService{},
}

// modelsWithEncryptedColumns names the structs in this package's source with a field tagged
// serializer:encrypted, so a new secret column cannot be missed by the test below.
func modelsWithEncryptedColumns(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var names []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				if field.Tag != nil && strings.Contains(field.Tag.Value, "serializer:"+encryptedSerializerName) {
					names = append(names, spec.Name.Name)
					break
				}
			}
			return true
		})
	}
	sort.Strings(names)
	return names
}

// The startup migration walks encryptedColumns; every column tagged with the encrypted
// serializer, in any model, must be in it, under the table name the serializer uses as label.
func TestEncryptedColumnsMatchModels(t *testing.T) {
	found := modelsWithEncryptedColumns(t)
	if len(found) == 0 {
		t.Fatal("found no model with an encrypted column")
	}
	for _, name := range found {
		if _, ok := secretModels[name]; !ok {
			t.Errorf("%s has an encrypted column: add it to secretModels and its columns to encryptedColumns", name)
		}
	}
	if len(found) != len(secretModels) {
		t.Errorf("models with encrypted columns %v, secretModels has %d", found, len(secretModels))
	}
	var tagged []string
	for _, m := range secretModels {
		s, err := schema.Parse(m, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range s.Fields {
			if _, ok := f.Serializer.(encryptedSerializer); ok {
				tagged = append(tagged, secretLabel(s.Table, f.DBName))
			}
		}
	}
	var listed []string
	for _, c := range encryptedColumns {
		listed = append(listed, secretLabel(c.table, c.column))
	}
	sort.Strings(tagged)
	sort.Strings(listed)
	if strings.Join(tagged, ",") != strings.Join(listed, ",") {
		t.Errorf("tagged columns %v, listed %v", tagged, listed)
	}
}

func TestChannelKeyEncryptedRoundTrip(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")

	if err := DB.Create(&Channel{Id: 1, Name: "a", Key: "sk-original-0001", ManageKey: strPtr("mk-manage-0001"), Models: "m"}).Error; err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", 1), "sk-original-0001")
	assertCiphertext(t, rawColumn(t, "channels", "manage_key", "id = ?", 1), "mk-manage-0001")

	var got Channel
	if err := DB.First(&got, 1).Error; err != nil {
		t.Fatal(err)
	}
	if got.Key != "sk-original-0001" || got.GetManageKey() != "mk-manage-0001" {
		t.Fatalf("loaded key=%q manage=%q", got.Key, got.GetManageKey())
	}

	// A form save with the key left empty keeps the stored key (and management key).
	ch := Channel{Id: 1, Name: "renamed", Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	if ch.Key != "sk-original-0001" || ch.GetManageKey() != "mk-manage-0001" || ch.Name != "renamed" {
		t.Errorf("after blank-key save: key=%q manage=%q name=%q", ch.Key, ch.GetManageKey(), ch.Name)
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", 1), "sk-original-0001")

	// A new key replaces it, encrypted.
	ch = Channel{Id: 1, Name: "renamed", Key: "sk-replaced-0002", Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", 1), "sk-replaced-0002")
	DB.First(&got, 1)
	if got.Key != "sk-replaced-0002" {
		t.Errorf("key after replace = %q", got.Key)
	}

	// A partial update (non-overwrite) without a key keeps it too.
	ch = Channel{Id: 1, Name: "partial"}
	if err := ch.UpdateRaw(false); err != nil {
		t.Fatal(err)
	}
	if ch.Key != "sk-replaced-0002" {
		t.Errorf("key after partial update = %q", ch.Key)
	}

	// An empty management key clears it and is stored as an empty string, not ciphertext.
	ch = Channel{Id: 1, Name: "partial", ManageKey: strPtr(""), Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	if stored := rawColumn(t, "channels", "manage_key", "id = ?", 1); stored != "" {
		t.Errorf("cleared manage key stored as %q", stored)
	}

	// Credential refresh write-back (OAuth channels) is encrypted as well.
	if err := UpdateChannelKey(1, `{"refresh_token":"rt-secret-0003"}`); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", 1), "rt-secret-0003")
	DB.First(&got, 1)
	if got.Key != `{"refresh_token":"rt-secret-0003"}` {
		t.Errorf("key after refresh write-back = %q", got.Key)
	}
}

// Batch inserts go through the serializer; the key search matches on the plaintext.
func TestChannelBatchInsertAndKeySearch(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")

	err := BatchInsert(DB, []Channel{
		{Id: 1, Name: "a", Key: "sk-search-a", Models: "m"},
		{Id: 2, Name: "b", Key: "sk-search-b", Models: "m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", 2), "sk-search-b")

	result, err := GetChannelsList(&SearchChannelsParams{Channel: Channel{Key: "sk-search-b"}, PaginationParams: PaginationParams{Page: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data == nil || len(*result.Data) != 1 || (*result.Data)[0].Id != 2 {
		t.Fatalf("key search found %+v", result.Data)
	}
	if (*result.Data)[0].Key != "" {
		t.Error("the list must not return the key")
	}
	result, err = GetChannelsList(&SearchChannelsParams{Channel: Channel{Key: "sk-none"}, PaginationParams: PaginationParams{Page: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data != nil && len(*result.Data) != 0 {
		t.Errorf("a missing key matched %d channels", len(*result.Data))
	}
}

// A tag edit matches keys by the md5 of the plaintext: unchanged keys keep their channel, new
// keys add encrypted channels, removed keys delete theirs.
func TestUpdateChannelsTagWithEncryptedKeys(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")
	for _, ch := range []Channel{
		{Id: 1, Name: "g_0", Tag: "grp", Key: "tag-key-aaaa", ManageKey: strPtr("tag-manage-1"), Models: "m", Status: 1},
		{Id: 2, Name: "g_1", Tag: "grp", Key: "tag-key-bbbb", Models: "m", Status: 1},
	} {
		if err := DB.Create(&ch).Error; err != nil {
			t.Fatal(err)
		}
	}

	tag, err := GetChannelsTag("grp")
	if err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum([]byte("tag-key-aaaa"))
	if tag.KeyMap[hex.EncodeToString(sum[:])] != 1 {
		t.Errorf("tag key map %v misses the plaintext md5 of channel 1", tag.KeyMap)
	}

	if err := UpdateChannelsTag("grp", &Channel{Tag: "grp", Name: "g", Models: "m2", Key: "tag-key-aaaa\ntag-key-cccc"}); err != nil {
		t.Fatal(err)
	}
	var got []Channel
	DB.Order("id").Find(&got)
	keys := map[string]Channel{}
	for _, ch := range got {
		keys[ch.Key] = ch
	}
	if len(got) != 2 {
		t.Fatalf("channels after tag edit: %d", len(got))
	}
	if ch, ok := keys["tag-key-aaaa"]; !ok || ch.Id != 1 || ch.Models != "m2" || ch.GetManageKey() != "tag-manage-1" {
		t.Errorf("kept channel = %+v", ch)
	}
	added, ok := keys["tag-key-cccc"]
	if !ok {
		t.Fatal("the new key did not add a channel")
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", added.Id), "tag-key-cccc")
	if _, ok := keys["tag-key-bbbb"]; ok {
		t.Error("the removed key's channel is still there")
	}
}

func TestPaymentConfigEncrypted(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")
	config := `{"secret_key":"sk_live_payment_secret","webhook_secret":"whsec_1"}`
	p := &Payment{Type: "stripe", Name: "Stripe", Config: config}
	if err := p.Insert(); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "payments", "config", "id = ?", p.ID), "sk_live_payment_secret")

	got, err := GetPaymentByID(p.ID)
	if err != nil || got.Config != config {
		t.Fatalf("payment config = %q, %v", got.Config, err)
	}
	got.Name = "Stripe 2"
	if err := got.Update(true); err != nil {
		t.Fatal(err)
	}
	again, _ := GetPaymentByID(p.ID)
	if again.Config != config || again.Name != "Stripe 2" {
		t.Errorf("after update: config=%q name=%q", again.Config, again.Name)
	}
	assertCiphertext(t, rawColumn(t, "payments", "config", "id = ?", p.ID), "sk_live_payment_secret")
}

func TestNotifyAndOidcSecretsEncrypted(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")

	n := &NotifyChannel{Name: "tg", Type: "telegram", Secrets: `{"bot_token":"123:telegram-secret"}`}
	if err := n.Insert(); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "notify_channels", "secrets", "id = ?", n.Id), "telegram-secret")
	n.Name = "tg2"
	if err := n.Update(); err != nil {
		t.Fatal(err)
	}
	loaded, err := GetNotifyChannelById(n.Id)
	if err != nil || loaded.Secrets != `{"bot_token":"123:telegram-secret"}` {
		t.Fatalf("notify secrets = %q, %v", loaded.Secrets, err)
	}

	// A client secret longer than the old varchar(500) still fits once encrypted.
	secret := strings.Repeat("s", 500)
	p := &OidcProvider{Slug: "idp", DisplayName: "IdP", Issuer: "https://idp.test", ClientId: "cid", ClientSecret: secret}
	if err := p.Insert(); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "oidc_providers", "client_secret", "id = ?", p.Id), secret)
	// An update without a secret keeps the stored one.
	p.ClientSecret = ""
	p.DisplayName = "IdP 2"
	if err := p.Update(); err != nil {
		t.Fatal(err)
	}
	got, err := GetOidcProviderById(p.Id)
	if err != nil || got.ClientSecret != secret || got.DisplayName != "IdP 2" {
		t.Fatalf("oidc provider secret kept=%v name=%q err=%v", got.ClientSecret == secret, got.DisplayName, err)
	}
}

func registerSecretOptionForTest(t *testing.T) {
	t.Helper()
	config.GlobalOption.RegisterString("SMTPToken", &config.SMTPToken, config.ScopeSecret)
	config.GlobalOption.RegisterString("SMTPServer", &config.SMTPServer, config.ScopeAdmin)
	oldToken, oldServer := config.SMTPToken, config.SMTPServer
	t.Cleanup(func() { config.SMTPToken, config.SMTPServer = oldToken, oldServer })
}

func TestSecretOptionEncrypted(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")
	registerSecretOptionForTest(t)

	if err := UpdateOption("SMTPToken", "smtp-secret-token"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateOption("SMTPServer", "smtp.example.test"); err != nil {
		t.Fatal(err)
	}
	if config.SMTPToken != "smtp-secret-token" {
		t.Errorf("running value = %q", config.SMTPToken)
	}
	assertCiphertext(t, rawColumn(t, "options", "value", quotePostgresField("key")+" = ?", "SMTPToken"), "smtp-secret-token")
	if stored := rawColumn(t, "options", "value", quotePostgresField("key")+" = ?", "SMTPServer"); stored != "smtp.example.test" {
		t.Errorf("a non-secret option is stored as %q", stored)
	}

	option, err := GetOption("SMTPToken")
	if err != nil || option.Value != "smtp-secret-token" {
		t.Fatalf("GetOption = %q, %v", option.Value, err)
	}
	config.SMTPToken = ""
	loadOptionsFromDatabase()
	if config.SMTPToken != "smtp-secret-token" {
		t.Errorf("value loaded from the database = %q", config.SMTPToken)
	}

	// A value no key opens is skipped on load: the running value stays.
	useEncryptionKey(t, secretTestKey(90), "")
	loadOptionsFromDatabase()
	if config.SMTPToken != "smtp-secret-token" {
		t.Errorf("an undecryptable option replaced the running value with %q", config.SMTPToken)
	}
}

// seedPlaintextSecrets writes one value per encrypted column with encryption off, as an
// installation from before ENCRYPTION_KEY was set has them.
func seedPlaintextSecrets(t *testing.T) {
	t.Helper()
	for _, ch := range []Channel{
		{Id: 1, Name: "a", Key: "sk-legacy-a", ManageKey: strPtr("mk-legacy-a"), Models: "m"},
		{Id: 2, Name: "b", Key: "sk-legacy-b", Models: "m"},
		{Id: 3, Name: "c", Key: "", Models: "m"},
	} {
		if err := DB.Create(&ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Create(&Payment{Type: "stripe", Name: "s", UUID: "u1", Config: `{"secret_key":"sk_legacy_pay"}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&NotifyChannel{Type: "telegram", Secrets: `{"bot_token":"legacy-bot"}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&OidcProvider{Slug: "legacy", ClientSecret: "legacy-client-secret"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&MailService{Server: "smtp.legacy.test", Port: 465, Account: "legacy", Token: "legacy-mail-password",
		TLSMode: "ssl", From: "noreply@legacy.test", Active: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Option{Key: "SMTPToken", Value: "legacy-smtp"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Option{Key: "SMTPServer", Value: "smtp.legacy.test"}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertSecretsReadable(t *testing.T) {
	t.Helper()
	var a, b Channel
	DB.First(&a, 1)
	DB.First(&b, 2)
	if a.Key != "sk-legacy-a" || a.GetManageKey() != "mk-legacy-a" || b.Key != "sk-legacy-b" {
		t.Errorf("channels read back key=%q manage=%q key=%q", a.Key, a.GetManageKey(), b.Key)
	}
	var p Payment
	DB.First(&p)
	var n NotifyChannel
	DB.First(&n)
	var o OidcProvider
	DB.First(&o)
	option, _ := GetOption("SMTPToken")
	if p.Config != `{"secret_key":"sk_legacy_pay"}` || n.Secrets != `{"bot_token":"legacy-bot"}` ||
		o.ClientSecret != "legacy-client-secret" || option.Value != "legacy-smtp" {
		t.Errorf("read back payment=%q notify=%q oidc=%q option=%q", p.Config, n.Secrets, o.ClientSecret, option.Value)
	}
	services, err := GetMailServices()
	if err != nil || len(services) != 1 || services[0].Client().Password != "legacy-mail-password" {
		t.Errorf("mail services read back %+v, %v", services, err)
	}
}

func TestPrepareStoredSecretsMigratesIdempotently(t *testing.T) {
	setupSecretStorageDB(t)
	registerSecretOptionForTest(t)
	seedPlaintextSecrets(t)

	useEncryptionKey(t, secretTestKey(1), "")
	report, err := prepareStoredSecrets(DB, true)
	if err != nil || report.err() != nil {
		t.Fatalf("first run: %v %v", err, report.err())
	}
	if n, detail := countSummary(report.encrypted); n != 8 {
		t.Errorf("first run encrypted %d (%s), want 8", n, detail)
	}
	assertCiphertext(t, rawColumn(t, "channels", "key", "id = ?", 1), "sk-legacy-a")
	assertCiphertext(t, rawColumn(t, "channels", "manage_key", "id = ?", 1), "mk-legacy-a")
	assertCiphertext(t, rawColumn(t, "payments", "config", "1 = 1"), "sk_legacy_pay")
	assertCiphertext(t, rawColumn(t, "notify_channels", "secrets", "1 = 1"), "legacy-bot")
	assertCiphertext(t, rawColumn(t, "oidc_providers", "client_secret", "1 = 1"), "legacy-client-secret")
	assertCiphertext(t, rawColumn(t, "mail_services", "token", "1 = 1"), "legacy-mail-password")
	assertCiphertext(t, rawColumn(t, "options", "value", quotePostgresField("key")+" = ?", "SMTPToken"), "legacy-smtp")
	if stored := rawColumn(t, "options", "value", quotePostgresField("key")+" = ?", "SMTPServer"); stored != "smtp.legacy.test" {
		t.Errorf("a non-secret option was rewritten to %q", stored)
	}
	if stored := rawColumn(t, "channels", "key", "id = ?", 3); stored != "" {
		t.Errorf("an empty key was rewritten to %q", stored)
	}
	if stored := rawColumn(t, "channels", "manage_key", "id = ?", 2); stored != "<nil>" {
		t.Errorf("a missing manage key was rewritten to %q", stored)
	}
	assertSecretsReadable(t)

	before := rawColumn(t, "channels", "key", "id = ?", 1)
	report, err = prepareStoredSecrets(DB, true)
	if err != nil || report.err() != nil {
		t.Fatalf("second run: %v %v", err, report.err())
	}
	if n, _ := countSummary(report.encrypted); n != 0 {
		t.Errorf("second run encrypted %d values again", n)
	}
	if after := rawColumn(t, "channels", "key", "id = ?", 1); after != before {
		t.Error("second run rewrote an encrypted value")
	}
	assertSecretsReadable(t)
}

func TestPrepareStoredSecretsWithoutKey(t *testing.T) {
	setupSecretStorageDB(t)
	registerSecretOptionForTest(t)
	seedPlaintextSecrets(t)

	report, err := prepareStoredSecrets(DB, true)
	if err != nil || report.err() != nil {
		t.Fatalf("no-key run: %v %v", err, report.err())
	}
	if stored := rawColumn(t, "channels", "key", "id = ?", 1); stored != "sk-legacy-a" {
		t.Errorf("without a key the value changed to %q", stored)
	}
	assertSecretsReadable(t)

	// Once something is encrypted, starting without the key fails instead of serving channels
	// without keys.
	useEncryptionKey(t, secretTestKey(1), "")
	if err := UpdateChannelKey(2, "sk-legacy-b"); err != nil {
		t.Fatal(err)
	}
	_ = encryption.Configure("", "")
	report, err = prepareStoredSecrets(DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.err() == nil || !strings.Contains(report.err().Error(), "ENCRYPTION_KEY is not set") {
		t.Errorf("ciphertext without key: err = %v", report.err())
	}
	if strings.Contains(report.err().Error(), "sk-legacy") {
		t.Error("the error message carries a secret")
	}
}

func TestPrepareStoredSecretsRotatesAndRejectsWrongKey(t *testing.T) {
	setupSecretStorageDB(t)
	registerSecretOptionForTest(t)
	seedPlaintextSecrets(t)
	oldKey, newKey := secretTestKey(1), secretTestKey(100)

	useEncryptionKey(t, oldKey, "")
	if _, err := prepareStoredSecrets(DB, true); err != nil {
		t.Fatal(err)
	}

	// A wrong key is refused.
	useEncryptionKey(t, newKey, "")
	report, err := prepareStoredSecrets(DB, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.err() == nil || !strings.Contains(report.err().Error(), "cannot be decrypted") {
		t.Errorf("wrong key: err = %v", report.err())
	}

	// A replica reads values under the previous key but leaves them for the master node.
	useEncryptionKey(t, newKey, oldKey)
	underOld := rawColumn(t, "channels", "key", "id = ?", 1)
	report, err = prepareStoredSecrets(DB, false)
	if err != nil || report.err() != nil {
		t.Fatalf("replica run: %v %v", err, report.err())
	}
	if rawColumn(t, "channels", "key", "id = ?", 1) != underOld {
		t.Error("a replica rewrote a stored value")
	}

	// The master node re-encrypts them under the new key.
	report, err = prepareStoredSecrets(DB, true)
	if err != nil || report.err() != nil {
		t.Fatalf("rotation run: %v %v", err, report.err())
	}
	if n, detail := countSummary(report.reencrypted); n != 8 {
		t.Errorf("rotation re-encrypted %d (%s), want 8", n, detail)
	}

	// The previous key is no longer needed.
	useEncryptionKey(t, newKey, "")
	assertSecretsReadable(t)
	report, _ = prepareStoredSecrets(DB, true)
	if report.err() != nil {
		t.Errorf("after rotation: %v", report.err())
	}
}

// A channel whose key cannot be decrypted fails only its own field: the other channels load.
func TestUndecryptableChannelKeyFailsOnlyItsRow(t *testing.T) {
	setupSecretStorageDB(t)
	useEncryptionKey(t, secretTestKey(1), "")
	if err := DB.Create(&Channel{Id: 1, Name: "a", Key: "sk-a", Models: "m"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Channel{Id: 2, Name: "b", Key: "sk-b", Models: "m"}).Error; err != nil {
		t.Fatal(err)
	}
	// Channel 2's key was written under another key.
	useEncryptionKey(t, secretTestKey(70), "")
	foreign, _ := encryption.Encrypt("sk-foreign", secretLabel("channels", "key"))
	DB.Exec("UPDATE channels SET "+quotePostgresField("key")+" = ? WHERE id = 2", foreign)
	useEncryptionKey(t, secretTestKey(1), "")

	var channels []Channel
	err := DB.Order("id").Find(&channels).Error
	if err == nil || !strings.Contains(err.Error(), "channels.key") {
		t.Errorf("err = %v", err)
	}
	if len(channels) != 2 || channels[0].Key != "sk-a" || channels[1].Key != "" {
		t.Errorf("loaded %+v", channels)
	}
}

// oidcProviderBeforeEncryption is oidc_providers as it was before client_secret was widened.
type oidcProviderBeforeEncryption struct {
	Id           int    `gorm:"primaryKey"`
	Slug         string `gorm:"type:varchar(32);not null;uniqueIndex:idx_oidc_providers_slug"`
	ClientSecret string `gorm:"type:varchar(500);default:''"`
}

func (oidcProviderBeforeEncryption) TableName() string { return "oidc_providers" }

// AutoMigrate widens an existing client_secret column for its ciphertext and keeps the rows.
func TestOidcClientSecretColumnWidened(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&oidcProviderBeforeEncryption{}); err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(&oidcProviderBeforeEncryption{Slug: "old", ClientSecret: "old-secret"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&OidcProvider{}); err != nil {
		t.Fatalf("widen client_secret: %v", err)
	}
	columns, err := testDB.Migrator().ColumnTypes(&OidcProvider{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "client_secret" {
			if typ, _ := c.ColumnType(); !strings.Contains(strings.ToLower(typ), "1024") {
				t.Errorf("client_secret type = %q", typ)
			}
		}
	}
	var got OidcProvider
	if err := testDB.First(&got, "slug = ?", "old").Error; err != nil || got.ClientSecret != "old-secret" {
		t.Errorf("row after widening: %q, %v", got.ClientSecret, err)
	}
}

// The mail service password is stored encrypted and reads back for sending: the service list,
// the mirror into the SMTP options that sending uses, and the move from the old SMTP options.
func TestMailServiceTokenEncrypted(t *testing.T) {
	db := setupMailServiceTestDB(t)
	useEncryptionKey(t, secretTestKey(1), "")

	s := newTestMailService("mail.example.test")
	if err := s.Insert(); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "mail_services", "token", "id = ?", s.Id), "secret-mail.example.test")
	s.FromName = "Example"
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "mail_services", "token", "id = ?", s.Id), "secret-mail.example.test")
	loaded, err := GetMailServiceById(s.Id)
	if err != nil || loaded.Client().Password != "secret-mail.example.test" {
		t.Fatalf("loaded service password = %q, %v", loaded.Client().Password, err)
	}
	if _, err := ActivateMailService(s.Id); err != nil {
		t.Fatal(err)
	}
	if config.SMTPToken != "secret-mail.example.test" {
		t.Errorf("sending uses password %q", config.SMTPToken)
	}
	assertCiphertext(t, rawColumn(t, "options", "value", quotePostgresField("key")+" = ?", "SMTPToken"), "secret-mail.example.test")

	// The move from the SMTP options writes the password encrypted.
	if err := db.Exec("DELETE FROM mail_services").Error; err != nil {
		t.Fatal(err)
	}
	if err := UpdateOption("SMTPToken", "re_migrated"); err != nil {
		t.Fatal(err)
	}
	if err := migrateMailServices().Migrate(db); err != nil {
		t.Fatal(err)
	}
	assertCiphertext(t, rawColumn(t, "mail_services", "token", "1 = 1"), "re_migrated")
	services, err := GetMailServices()
	if err != nil || len(services) != 1 || services[0].Token != "re_migrated" {
		t.Fatalf("migrated services %+v, %v", services, err)
	}
}
