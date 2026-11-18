package model

import (
	"errors"
	"strconv"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/media"
	"github.com/modeltaps/modeltaps/common/storage"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func init() {
	// Deferred media links (common/media) fetch through the proxy of the channel that made them.
	media.ProxyForChannel = func(id int) string {
		if id <= 0 {
			return ""
		}
		ch, err := GetChannelById(id)
		if err != nil || ch.Proxy == nil {
			return ""
		}
		ch.SetProxy()
		return *ch.Proxy
	}
}

func registerImageStorageOptions() {
	o := config.GlobalOption
	o.RegisterString("ImageStorageProvider", &config.ImageStorageProvider, config.ScopeAdmin)
	o.RegisterString("ImageStorageS3Endpoint", &config.ImageStorageS3Endpoint, config.ScopeAdmin)
	o.RegisterString("ImageStorageS3Region", &config.ImageStorageS3Region, config.ScopeAdmin)
	o.RegisterString("ImageStorageS3AccessKeyId", &config.ImageStorageS3AccessKeyId, config.ScopeAdmin)
	o.RegisterString("ImageStorageS3AccessKeySecret", &config.ImageStorageS3AccessKeySecret, config.ScopeSecret)
	o.RegisterString("ImageStorageS3BucketName", &config.ImageStorageS3BucketName, config.ScopeAdmin)
	o.RegisterString("ImageStorageS3CDNURL", &config.ImageStorageS3CDNURL, config.ScopeAdmin)
	o.RegisterInt("ImageStorageS3ExpirationDays", &config.ImageStorageS3ExpirationDays, config.ScopeAdmin)
	o.RegisterString("ImageStorageOSSEndpoint", &config.ImageStorageOSSEndpoint, config.ScopeAdmin)
	o.RegisterString("ImageStorageOSSRegion", &config.ImageStorageOSSRegion, config.ScopeAdmin)
	o.RegisterString("ImageStorageOSSAccessKeyId", &config.ImageStorageOSSAccessKeyId, config.ScopeAdmin)
	o.RegisterString("ImageStorageOSSAccessKeySecret", &config.ImageStorageOSSAccessKeySecret, config.ScopeSecret)
	o.RegisterString("ImageStorageOSSBucketName", &config.ImageStorageOSSBucketName, config.ScopeAdmin)
	o.RegisterString("ImageStorageSMMSSecret", &config.ImageStorageSMMSSecret, config.ScopeSecret)
	o.RegisterString("ImageStorageImgurClientId", &config.ImageStorageImgurClientId, config.ScopeSecret)
	o.RegisterBool("ImageStorageRehostUpstream", &config.ImageStorageRehostUpstream, config.ScopeAdmin)
}

// legacyImageStorageOptions reads the image storage of the config file's old `storage:` section
// as the options that hold it now. Several drives could be set there and the first that worked
// was used, in no fixed order; one is kept, preferring S3, then OSS, SM.MS and Imgur.
func legacyImageStorageOptions(get func(string) string) map[string]string {
	s3 := map[string]string{
		"ImageStorageS3Endpoint":        get("storage.s3.endpoint"),
		"ImageStorageS3Region":          get("storage.s3.region"),
		"ImageStorageS3AccessKeyId":     get("storage.s3.accessKeyId"),
		"ImageStorageS3AccessKeySecret": get("storage.s3.accessKeySecret"),
		"ImageStorageS3BucketName":      get("storage.s3.bucketName"),
		"ImageStorageS3CDNURL":          get("storage.s3.cdnurl"),
	}
	if days, err := strconv.Atoi(get("storage.s3.expirationDays")); err == nil && days > 0 {
		s3["ImageStorageS3ExpirationDays"] = strconv.Itoa(days)
	}
	oss := map[string]string{
		"ImageStorageOSSEndpoint":        get("storage.alioss.endpoint"),
		"ImageStorageOSSRegion":          get("storage.alioss.region"),
		"ImageStorageOSSAccessKeyId":     get("storage.alioss.accessKeyId"),
		"ImageStorageOSSAccessKeySecret": get("storage.alioss.accessKeySecret"),
		"ImageStorageOSSBucketName":      get("storage.alioss.bucketName"),
	}
	candidates := []struct {
		provider string
		values   map[string]string
		required []string
	}{
		{storage.ProviderS3, s3, []string{"ImageStorageS3Endpoint", "ImageStorageS3AccessKeyId", "ImageStorageS3AccessKeySecret", "ImageStorageS3BucketName"}},
		{storage.ProviderAliOSS, oss, []string{"ImageStorageOSSEndpoint", "ImageStorageOSSAccessKeyId", "ImageStorageOSSAccessKeySecret", "ImageStorageOSSBucketName"}},
		{storage.ProviderSMMS, map[string]string{"ImageStorageSMMSSecret": get("storage.smms.secret")}, []string{"ImageStorageSMMSSecret"}},
		{storage.ProviderImgur, map[string]string{"ImageStorageImgurClientId": get("storage.imgur.client_id")}, []string{"ImageStorageImgurClientId"}},
	}
	for _, c := range candidates {
		complete := true
		for _, key := range c.required {
			if c.values[key] == "" {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		out := map[string]string{"ImageStorageProvider": c.provider}
		for k, v := range c.values {
			if v != "" {
				out[k] = v
			}
		}
		return out
	}
	return nil
}

// MigrateImageStorageFromConfig moves the image storage from the config file's `storage:`
// section into the platform settings, once: it runs while the settings have never held an
// image storage. It needs the registered options, so secret fields are stored encrypted.
func MigrateImageStorageFromConfig() {
	if _, err := GetOption("ImageStorageProvider"); err == nil {
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.SysError("image storage: reading the settings failed: " + err.Error())
		return
	}
	values := legacyImageStorageOptions(viper.GetString)
	if values == nil {
		return
	}
	// The provider goes last: until it is written the move is not done and runs again.
	for key, value := range values {
		if key == "ImageStorageProvider" {
			continue
		}
		if err := UpdateOption(key, value); err != nil {
			logger.SysError("image storage: moving the config file settings failed: " + err.Error())
			return
		}
	}
	if err := UpdateOption("ImageStorageProvider", values["ImageStorageProvider"]); err != nil {
		logger.SysError("image storage: moving the config file settings failed: " + err.Error())
		return
	}
	logger.SysLog("image storage (" + values["ImageStorageProvider"] + ") moved from the config file into the platform settings; the config file's storage section is no longer read")
}
