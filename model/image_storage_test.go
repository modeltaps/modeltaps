package model

import (
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/spf13/viper"
)

func TestLegacyImageStorageOptions(t *testing.T) {
	from := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	if got := legacyImageStorageOptions(from(nil)); got != nil {
		t.Fatalf("nothing set = %v, want nil", got)
	}
	// An incomplete S3 section is skipped in favour of a complete OSS one.
	got := legacyImageStorageOptions(from(map[string]string{
		"storage.s3.endpoint":            "https://r2.example.com",
		"storage.alioss.endpoint":        "oss-cn-beijing.aliyuncs.com",
		"storage.alioss.accessKeyId":     "id",
		"storage.alioss.accessKeySecret": "secret",
		"storage.alioss.bucketName":      "bucket",
		"storage.smms.secret":            "sm",
	}))
	want := map[string]string{
		"ImageStorageProvider":           "alioss",
		"ImageStorageOSSEndpoint":        "oss-cn-beijing.aliyuncs.com",
		"ImageStorageOSSAccessKeyId":     "id",
		"ImageStorageOSSAccessKeySecret": "secret",
		"ImageStorageOSSBucketName":      "bucket",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// S3 wins when complete, and carries its optional fields.
	got = legacyImageStorageOptions(from(map[string]string{
		"storage.s3.endpoint":        "https://r2.example.com",
		"storage.s3.accessKeyId":     "id",
		"storage.s3.accessKeySecret": "secret",
		"storage.s3.bucketName":      "bucket",
		"storage.s3.cdnurl":          "https://img.example.com",
		"storage.s3.expirationDays":  "7",
		"storage.imgur.client_id":    "c",
	}))
	if got["ImageStorageProvider"] != "s3" || got["ImageStorageS3CDNURL"] != "https://img.example.com" || got["ImageStorageS3ExpirationDays"] != "7" {
		t.Fatalf("got %v", got)
	}
}

func TestMigrateImageStorageFromConfigRunsOnce(t *testing.T) {
	testDB := setupMailServiceTestDB(t)
	savedProvider, savedKey := config.ImageStorageProvider, config.ImageStorageImgurClientId
	t.Cleanup(func() {
		config.ImageStorageProvider, config.ImageStorageImgurClientId = savedProvider, savedKey
		viper.Set("storage.imgur.client_id", nil)
	})

	viper.Set("storage.imgur.client_id", "from-config")
	MigrateImageStorageFromConfig()
	if config.ImageStorageProvider != "imgur" || config.ImageStorageImgurClientId != "from-config" {
		t.Fatalf("after the move: provider %q, client id %q", config.ImageStorageProvider, config.ImageStorageImgurClientId)
	}

	// Turned off in the settings afterwards: the config file is not read again.
	if err := UpdateOption("ImageStorageProvider", ""); err != nil {
		t.Fatal(err)
	}
	viper.Set("storage.imgur.client_id", "changed")
	MigrateImageStorageFromConfig()
	var o Option
	if err := testDB.First(&o, "key = ?", "ImageStorageProvider").Error; err != nil || o.Value != "" {
		t.Fatalf("provider row = %q, %v; want it left off", o.Value, err)
	}
	if config.ImageStorageImgurClientId != "from-config" {
		t.Fatalf("client id = %q, want the first move's value", config.ImageStorageImgurClientId)
	}
}
