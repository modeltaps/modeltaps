// Package storage uploads generated images (Cloudflare Workers AI, Stability AI, Gemini) to the
// image storage set in the platform settings, so a request that asks for an image URL gets one.
// By default the gateway keeps them itself for an hour (common/media). Turned off, Upload returns
// "" and callers answer with base64 instead.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/media"
	"github.com/modeltaps/modeltaps/common/storage/drives"
)

type StorageDrive interface {
	Upload(data []byte, fileName string) (string, error)
	Name() string
}

const (
	// ProviderLocal keeps images on the gateway for an hour (the default; "" means the same).
	ProviderLocal  = "local"
	ProviderOff    = "off"
	ProviderS3     = "s3"
	ProviderAliOSS = "alioss"
	ProviderSMMS   = "smms"
	ProviderImgur  = "imgur"
)

// ValidProvider reports whether p is a provider the settings accept ("" turns storage off).
func ValidProvider(p string) bool {
	switch p {
	case "", ProviderLocal, ProviderOff, ProviderS3, ProviderAliOSS, ProviderSMMS, ProviderImgur:
		return true
	}
	return false
}

type S3Settings struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	AccessKeyId     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	BucketName      string `json:"bucket_name"`
	CDNURL          string `json:"cdn_url"`
	ExpirationDays  int    `json:"expiration_days"`
}

type OSSSettings struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	AccessKeyId     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	BucketName      string `json:"bucket_name"`
}

// Settings is one image storage configuration: the provider in use and the fields of each.
type Settings struct {
	Provider      string      `json:"provider"`
	S3            S3Settings  `json:"s3"`
	OSS           OSSSettings `json:"alioss"`
	SMMSSecret    string      `json:"smms_secret"`
	ImgurClientId string      `json:"imgur_client_id"`
}

var (
	ErrStorageOff        = errors.New("image storage is off")
	ErrStorageProvider   = errors.New("unknown image storage provider")
	ErrStorageIncomplete = errors.New("fill in every required field of the image storage")
	ErrStorageOSSRegion  = errors.New("the OSS region cannot be derived from the endpoint; enter it, for example cn-beijing")
)

// CurrentSettings returns the image storage as saved in the platform settings.
func CurrentSettings() Settings {
	return Settings{
		Provider: config.ImageStorageProvider,
		S3: S3Settings{
			Endpoint:        config.ImageStorageS3Endpoint,
			Region:          config.ImageStorageS3Region,
			AccessKeyId:     config.ImageStorageS3AccessKeyId,
			AccessKeySecret: config.ImageStorageS3AccessKeySecret,
			BucketName:      config.ImageStorageS3BucketName,
			CDNURL:          config.ImageStorageS3CDNURL,
			ExpirationDays:  config.ImageStorageS3ExpirationDays,
		},
		OSS: OSSSettings{
			Endpoint:        config.ImageStorageOSSEndpoint,
			Region:          config.ImageStorageOSSRegion,
			AccessKeyId:     config.ImageStorageOSSAccessKeyId,
			AccessKeySecret: config.ImageStorageOSSAccessKeySecret,
			BucketName:      config.ImageStorageOSSBucketName,
		},
		SMMSSecret:    config.ImageStorageSMMSSecret,
		ImgurClientId: config.ImageStorageImgurClientId,
	}
}

func filled(values ...string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return false
		}
	}
	return true
}

// Drive builds the uploader for the settings, or says what is missing.
func (s Settings) Drive() (StorageDrive, error) {
	switch s.Provider {
	case "", ProviderLocal:
		return gatewayDrive{}, nil
	case ProviderOff:
		return nil, ErrStorageOff
	case ProviderS3:
		c := s.S3
		if !filled(c.Endpoint, c.AccessKeyId, c.AccessKeySecret, c.BucketName) {
			return nil, ErrStorageIncomplete
		}
		return drives.NewS3Upload(strings.TrimSpace(c.Endpoint), c.Region, strings.TrimSpace(c.AccessKeyId), c.AccessKeySecret,
			strings.TrimSpace(c.BucketName), c.CDNURL, c.ExpirationDays), nil
	case ProviderAliOSS:
		c := s.OSS
		if !filled(c.Endpoint, c.AccessKeyId, c.AccessKeySecret, c.BucketName) {
			return nil, ErrStorageIncomplete
		}
		d := drives.NewAliOSSUpload(strings.TrimSpace(c.Endpoint), strings.TrimSpace(c.Region), strings.TrimSpace(c.AccessKeyId),
			c.AccessKeySecret, strings.TrimSpace(c.BucketName))
		if d.Region == "" {
			return nil, ErrStorageOSSRegion
		}
		return d, nil
	case ProviderSMMS:
		if !filled(s.SMMSSecret) {
			return nil, ErrStorageIncomplete
		}
		return drives.NewSMUpload(strings.TrimSpace(s.SMMSSecret)), nil
	case ProviderImgur:
		if !filled(s.ImgurClientId) {
			return nil, ErrStorageIncomplete
		}
		return drives.NewImgurUpload(strings.TrimSpace(s.ImgurClientId)), nil
	}
	return nil, ErrStorageProvider
}

// gatewayDrive keeps the image on the gateway (common/media).
type gatewayDrive struct{}

func (gatewayDrive) Name() string { return "Gateway" }

func (gatewayDrive) Upload(data []byte, _ string) (string, error) { return media.Put(data) }

// UploadWith uploads data with the given settings and returns the image URL.
func UploadWith(s Settings, data []byte, fileName string) (string, error) {
	drive, err := s.Drive()
	if err != nil {
		return "", err
	}
	url, err := drive.Upload(data, fileName)
	if err != nil {
		return "", fmt.Errorf("%s: %w", drive.Name(), err)
	}
	if url == "" {
		return "", fmt.Errorf("%s returned no URL", drive.Name())
	}
	return url, nil
}

// Upload uploads data to the image storage in use and returns its URL, or "" when storage is
// off or the upload failed (logged); the caller then answers with base64.
func Upload(data []byte, fileName string) string {
	url, err := UploadWith(CurrentSettings(), data, fileName)
	if err != nil {
		if !errors.Is(err, ErrStorageOff) {
			//lint:ignore SA1029 the logger reads the request id under this key
			ctx := context.WithValue(context.Background(), logger.RequestIdKey, "Upload")
			logger.LogError(ctx, "image storage upload failed, answering with base64: "+err.Error())
		}
		return ""
	}
	return url
}
