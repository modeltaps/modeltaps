package controller

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/modeltaps/modeltaps/common/storage"
)

func TestImageStorageTestSettingsReusesSavedSecretOnlyForSavedTarget(t *testing.T) {
	saved := storage.Settings{
		Provider: storage.ProviderS3,
		S3:       storage.S3Settings{Endpoint: "https://r2.example.com", AccessKeyId: "id", AccessKeySecret: "saved", BucketName: "b"},
		OSS:      storage.OSSSettings{Endpoint: "oss-cn-beijing.aliyuncs.com", AccessKeyId: "oid", AccessKeySecret: "osaved", BucketName: "ob"},
	}

	same := imageStorageTestSettings(storage.Settings{Provider: storage.ProviderS3,
		S3: storage.S3Settings{Endpoint: "https://R2.example.com ", AccessKeyId: "id", BucketName: "b"}}, saved)
	if same.S3.AccessKeySecret != "saved" {
		t.Fatalf("same target: secret = %q, want the saved one", same.S3.AccessKeySecret)
	}

	for name, s3 := range map[string]storage.S3Settings{
		"other endpoint": {Endpoint: "https://evil.example.com", AccessKeyId: "id", BucketName: "b"},
		"other bucket":   {Endpoint: "https://r2.example.com", AccessKeyId: "id", BucketName: "other"},
		"other key id":   {Endpoint: "https://r2.example.com", AccessKeyId: "other", BucketName: "b"},
	} {
		got := imageStorageTestSettings(storage.Settings{Provider: storage.ProviderS3, S3: s3}, saved)
		if got.S3.AccessKeySecret != "" {
			t.Fatalf("%s: the saved secret was handed over", name)
		}
	}

	typed := imageStorageTestSettings(storage.Settings{Provider: storage.ProviderAliOSS,
		OSS: storage.OSSSettings{Endpoint: "oss-cn-beijing.aliyuncs.com", AccessKeyId: "oid", AccessKeySecret: "new", BucketName: "ob"}}, saved)
	if typed.OSS.AccessKeySecret != "new" {
		t.Fatalf("a typed secret must be used as is, got %q", typed.OSS.AccessKeySecret)
	}
}

func TestImageStorageTestPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(imageStorageTestPNG()))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("size = %v", b)
	}
}
