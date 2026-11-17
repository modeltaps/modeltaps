package model

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/encryption"
	"github.com/modeltaps/modeltaps/common/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Stored credentials are encrypted at rest when ENCRYPTION_KEY is set (see common/encryption).
// Columns tagged `serializer:encrypted` are encrypted on write and decrypted on read by GORM, so
// callers keep working with plaintext (channel.Key, payment.Config ...). The serializer only runs
// for struct writes (Create, Save, Updates with a struct) and struct reads (Find, First, Scan into
// the model). Writes through a map or Update("column", value) store the value as given, and SQL
// conditions compare against the stored text, so:
//   - write these columns with a struct (Select("column").Updates(&Model{...})), never a map;
//   - never filter on them in SQL (key = ?, LIKE): ciphertext differs on every write. Load the
//     rows and compare in Go instead (see channelIDsWithKey).
// Legacy plaintext (written before the key was set, or by a map write) reads back unchanged and
// is encrypted by PrepareStoredSecrets at the next start.
//
// The label bound to each ciphertext is "<table>.<column>": renaming the table or the column makes
// the stored values unreadable unless they are re-encrypted under the new label.

const encryptedSerializerName = "encrypted"

func init() {
	schema.RegisterSerializer(encryptedSerializerName, encryptedSerializer{})
}

// encryptedColumns lists every column stored through the encrypted serializer. The startup
// migration walks these; TestEncryptedColumnsMatchModels keeps the list and the model tags in step.
var encryptedColumns = []struct {
	table  string
	column string
}{
	{"channels", "key"},
	{"channels", "manage_key"},
	{"payments", "config"},
	{"notify_channels", "secrets"},
	{"oidc_providers", "client_secret"},
}

func secretLabel(table, column string) string { return table + "." + column }

// optionSecretLabel binds an encrypted option value to its option key.
func optionSecretLabel(key string) string { return "options." + key }

type encryptedSerializer struct{}

// Scan decrypts the stored value into a string or *string field.
func (encryptedSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) error {
	target := field.ReflectValueOf(ctx, dst)
	var stored string
	switch v := dbValue.(type) {
	case nil:
		target.Set(reflect.Zero(field.FieldType))
		return nil
	case string:
		stored = v
	case []byte:
		stored = string(v)
	default:
		return fmt.Errorf("%s: unsupported stored type %T", secretLabel(field.Schema.Table, field.DBName), dbValue)
	}
	label := secretLabel(field.Schema.Table, field.DBName)
	plaintext, err := encryption.Decrypt(stored, label)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	switch field.FieldType.Kind() {
	case reflect.String:
		target.SetString(plaintext)
	case reflect.Pointer:
		target.Set(reflect.ValueOf(&plaintext))
	default:
		return fmt.Errorf("%s: the encrypted serializer needs a string field", label)
	}
	return nil
}

// Value encrypts a string or *string field for storage. Empty strings and nil pointers are stored
// as they are, so "has no key" conditions (manage_key <> ”) keep working.
func (encryptedSerializer) Value(_ context.Context, field *schema.Field, _ reflect.Value, fieldValue interface{}) (interface{}, error) {
	label := secretLabel(field.Schema.Table, field.DBName)
	switch v := fieldValue.(type) {
	case string:
		return encryption.Encrypt(v, label)
	case *string:
		if v == nil {
			return nil, nil
		}
		return encryption.Encrypt(*v, label)
	default:
		return nil, fmt.Errorf("%s: the encrypted serializer needs a string field, got %T", label, fieldValue)
	}
}

// secretStorageReport counts the stored credentials by state, per "<table>.<column>".
type secretStorageReport struct {
	encrypted     map[string]int // plaintext encrypted now
	reencrypted   map[string]int // moved from the previous key to the current one
	plaintext     map[string]int // left in plaintext (no key, or not the master node)
	previous      map[string]int // still under the previous key (not the master node)
	undecryptable map[string]int
}

func newSecretStorageReport() *secretStorageReport {
	return &secretStorageReport{
		encrypted: map[string]int{}, reencrypted: map[string]int{}, plaintext: map[string]int{},
		previous: map[string]int{}, undecryptable: map[string]int{},
	}
}

// PrepareStoredSecrets checks the stored credentials against ENCRYPTION_KEY at startup and, on
// the master node with a key set, encrypts the values still in plaintext and re-encrypts the ones
// under ENCRYPTION_KEY_PREVIOUS. It is idempotent. It fails when a stored value is encrypted but
// cannot be decrypted (no key, or the wrong key): starting anyway would leave channels without
// keys. Call it after the options are registered (InitOptionMap), so secret options are known.
func PrepareStoredSecrets() error {
	report, err := prepareStoredSecrets(DB, config.IsMasterNode)
	if err != nil {
		return err
	}
	report.log()
	return report.err()
}

func prepareStoredSecrets(db *gorm.DB, write bool) (*secretStorageReport, error) {
	report := newSecretStorageReport()
	for _, c := range encryptedColumns {
		if err := prepareSecretColumn(db, c.table, c.column, write, report); err != nil {
			return report, err
		}
	}
	if err := prepareSecretOptions(db, write, report); err != nil {
		return report, err
	}
	return report, nil
}

const secretMigrationBatch = 200

// prepareSecretColumn walks one table in id order, in batches. Rows are read and written by
// column name, past the serializer, so the stored text is seen as it is.
func prepareSecretColumn(db *gorm.DB, table, column string, write bool, report *secretStorageReport) error {
	if !db.Migrator().HasTable(table) || !db.Migrator().HasColumn(table, column) {
		return nil
	}
	label := secretLabel(table, column)
	quoted := quotePostgresField(column)
	lastID := 0
	for {
		var rows []struct {
			Id     int
			Stored *string
		}
		err := db.Table(table).Select("id, "+quoted+" AS stored").
			Where("id > ? AND "+quoted+" IS NOT NULL AND "+quoted+" <> ''", lastID).
			Order("id").Limit(secretMigrationBatch).Find(&rows).Error
		if err != nil {
			return fmt.Errorf("read %s: %w", label, err)
		}
		for _, row := range rows {
			lastID = row.Id
			if row.Stored == nil {
				continue
			}
			rewrite, err := report.classify(label, *row.Stored, write)
			if err != nil {
				return err
			}
			if rewrite == "" {
				continue
			}
			// The old value in the condition skips a row someone changed meanwhile.
			err = db.Table(table).Where("id = ? AND "+quoted+" = ?", row.Id, *row.Stored).
				UpdateColumn(column, rewrite).Error
			if err != nil {
				return fmt.Errorf("encrypt %s of row %d: %w", label, row.Id, err)
			}
		}
		if len(rows) < secretMigrationBatch {
			return nil
		}
	}
}

// prepareSecretOptions handles the ScopeSecret options (SMTP token, OAuth client secrets ...).
func prepareSecretOptions(db *gorm.DB, write bool, report *secretStorageReport) error {
	if !db.Migrator().HasTable(&Option{}) {
		return nil
	}
	keys := config.GlobalOption.SecretKeys()
	if len(keys) == 0 {
		return nil
	}
	// Read every option and filter in Go: the column name key is quoted differently per
	// database, and the table holds a few dozen rows. Table() skips the Option hooks, so the
	// stored text is seen as it is.
	var rows []struct {
		Key   string
		Value string
	}
	if err := db.Table("options").Select("*").Find(&rows).Error; err != nil {
		return fmt.Errorf("read options: %w", err)
	}
	keyCol, valueCol := quotePostgresField("key"), quotePostgresField("value")
	for _, row := range rows {
		if !config.GlobalOption.IsSecret(row.Key) || row.Value == "" {
			continue
		}
		rewrite, err := report.classifyAs("options", optionSecretLabel(row.Key), row.Value, write)
		if err != nil {
			return err
		}
		if rewrite == "" {
			continue
		}
		err = db.Table("options").Where(keyCol+" = ? AND "+valueCol+" = ?", row.Key, row.Value).
			UpdateColumn("value", rewrite).Error
		if err != nil {
			return fmt.Errorf("encrypt option %s: %w", row.Key, err)
		}
	}
	return nil
}

func (r *secretStorageReport) classify(label, stored string, write bool) (string, error) {
	return r.classifyAs(label, label, stored, write)
}

// classifyAs counts stored under name and returns the value to write back, or "" to leave it.
func (r *secretStorageReport) classifyAs(name, label, stored string, write bool) (string, error) {
	state, plaintext := encryption.Inspect(stored, label)
	switch state {
	case encryption.StatePlaintext:
		if !write || !encryption.Enabled() {
			r.plaintext[name]++
			return "", nil
		}
		r.encrypted[name]++
	case encryption.StatePrevious:
		if !write {
			r.previous[name]++
			return "", nil
		}
		r.reencrypted[name]++
	case encryption.StateUndecryptable:
		r.undecryptable[name]++
		return "", nil
	default:
		return "", nil
	}
	rewrite, err := encryption.Encrypt(plaintext, label)
	if err != nil {
		return "", fmt.Errorf("encrypt %s: %w", label, err)
	}
	return rewrite, nil
}

func countSummary(counts map[string]int) (int, string) {
	names := make([]string, 0, len(counts))
	total := 0
	for name, n := range counts {
		names = append(names, fmt.Sprintf("%s: %d", name, n))
		total += n
	}
	sort.Strings(names)
	return total, strings.Join(names, ", ")
}

func (r *secretStorageReport) log() {
	if n, detail := countSummary(r.encrypted); n > 0 {
		logger.SysLog(fmt.Sprintf("encrypted %d stored secrets that were in plaintext (%s)", n, detail))
	}
	if n, detail := countSummary(r.reencrypted); n > 0 {
		logger.SysLog(fmt.Sprintf("re-encrypted %d stored secrets from ENCRYPTION_KEY_PREVIOUS to ENCRYPTION_KEY (%s); "+
			"ENCRYPTION_KEY_PREVIOUS can be removed once every node runs with the new key", n, detail))
	}
	if n, detail := countSummary(r.previous); n > 0 {
		logger.SysLog(fmt.Sprintf("%d stored secrets are still under ENCRYPTION_KEY_PREVIOUS (%s); the master node re-encrypts them", n, detail))
	}
	if n, detail := countSummary(r.plaintext); n > 0 && encryption.Enabled() {
		logger.SysLog(fmt.Sprintf("%d stored secrets are still in plaintext (%s); the master node encrypts them", n, detail))
	}
}

func (r *secretStorageReport) err() error {
	n, detail := countSummary(r.undecryptable)
	if n == 0 {
		return nil
	}
	if !encryption.Enabled() {
		return fmt.Errorf("%d stored secrets are encrypted but ENCRYPTION_KEY is not set (%s); "+
			"start with the ENCRYPTION_KEY they were written with", n, detail)
	}
	return fmt.Errorf("%d stored secrets cannot be decrypted with ENCRYPTION_KEY or ENCRYPTION_KEY_PREVIOUS (%s); "+
		"start with the key they were written with, set the old key as ENCRYPTION_KEY_PREVIOUS when rotating, "+
		"or clear and re-enter those values", n, detail)
}
