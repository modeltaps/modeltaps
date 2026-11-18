package storage

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestSettingsDrive(t *testing.T) {
	cases := []struct {
		name string
		s    Settings
		want error
		kind string
	}{
		{"default is the gateway", Settings{}, nil, "Gateway"},
		{"local", Settings{Provider: ProviderLocal}, nil, "Gateway"},
		{"off", Settings{Provider: ProviderOff}, ErrStorageOff, ""},
		{"unknown", Settings{Provider: "ftp"}, ErrStorageProvider, ""},
		{"s3 missing secret", Settings{Provider: ProviderS3, S3: S3Settings{Endpoint: "https://s3.us-east-1.amazonaws.com", AccessKeyId: "id", BucketName: "b"}}, ErrStorageIncomplete, ""},
		{"s3", Settings{Provider: ProviderS3, S3: S3Settings{Endpoint: "https://s3.us-east-1.amazonaws.com", AccessKeyId: "id", AccessKeySecret: "k", BucketName: "b"}}, nil, "S3"},
		{"oss no region", Settings{Provider: ProviderAliOSS, OSS: OSSSettings{Endpoint: "https://oss.example.com", AccessKeyId: "id", AccessKeySecret: "k", BucketName: "b"}}, ErrStorageOSSRegion, ""},
		{"oss derived region", Settings{Provider: ProviderAliOSS, OSS: OSSSettings{Endpoint: "oss-cn-beijing.aliyuncs.com", AccessKeyId: "id", AccessKeySecret: "k", BucketName: "b"}}, nil, "AliOSS"},
		{"smms blank", Settings{Provider: ProviderSMMS, SMMSSecret: "  "}, ErrStorageIncomplete, ""},
		{"imgur", Settings{Provider: ProviderImgur, ImgurClientId: "c"}, nil, "Imgur"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := c.s.Drive()
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if c.want == nil && d.Name() != c.kind {
				t.Fatalf("drive = %s, want %s", d.Name(), c.kind)
			}
		})
	}
}

func TestUploadIsEmptyWhenOff(t *testing.T) {
	saved := config.ImageStorageProvider
	config.ImageStorageProvider = ProviderOff
	t.Cleanup(func() { config.ImageStorageProvider = saved })
	if url := Upload([]byte("x"), "a.png"); url != "" {
		t.Fatalf("Upload with storage off = %q, want empty", url)
	}
}
