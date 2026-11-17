package storage

import (
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/storage/drives"

	"github.com/spf13/viper"
)

type Storage struct {
	drives map[string]StorageDrive
}

func InitStorage() {
	InitImgurStorage()
	InitSMStorage()
	InitALIOSSStorage()
	InitS3Storage()
}

func InitALIOSSStorage() {
	endpoint := viper.GetString("storage.alioss.endpoint")
	if endpoint == "" {
		return
	}
	accessKeyId := viper.GetString("storage.alioss.accessKeyId")
	if accessKeyId == "" {
		return
	}
	accessKeySecret := viper.GetString("storage.alioss.accessKeySecret")
	if accessKeySecret == "" {
		return
	}
	bucketName := viper.GetString("storage.alioss.bucketName")
	if bucketName == "" {
		return
	}

	region := viper.GetString("storage.alioss.region")
	aliUpload := drives.NewAliOSSUpload(endpoint, region, accessKeyId, accessKeySecret, bucketName)
	if aliUpload.Region == "" {
		logger.SysError("storage.alioss: region is not set and cannot be derived from endpoint " + endpoint + "; set storage.alioss.region (for example cn-beijing), OSS storage stays off")
		return
	}
	AddStorageDrive(aliUpload)
}

func InitSMStorage() {
	smSecret := viper.GetString("storage.smms.secret")
	if smSecret == "" {
		return
	}

	smUpload := drives.NewSMUpload(smSecret)
	AddStorageDrive(smUpload)
}

func InitImgurStorage() {
	imgurClientId := viper.GetString("storage.imgur.client_id")
	if imgurClientId == "" {
		return
	}

	imgurUpload := drives.NewImgurUpload(imgurClientId)
	AddStorageDrive(imgurUpload)
}

func InitS3Storage() {
	endpoint := viper.GetString("storage.s3.endpoint")
	if endpoint == "" {
		return
	}
	accessKeyId := viper.GetString("storage.s3.accessKeyId")
	if accessKeyId == "" {
		return
	}
	accessKeySecret := viper.GetString("storage.s3.accessKeySecret")
	if accessKeySecret == "" {
		return
	}
	bucketName := viper.GetString("storage.s3.bucketName")
	if bucketName == "" {
		return
	}
	cdnurl := viper.GetString("storage.s3.cdnurl")
	if cdnurl == "" {
		cdnurl = endpoint
	}

	expirationDays := viper.GetInt("storage.s3.expirationDays")

	region := viper.GetString("storage.s3.region")
	s3Upload := drives.NewS3Upload(endpoint, region, accessKeyId, accessKeySecret, bucketName, cdnurl, expirationDays)
	AddStorageDrive(s3Upload)
}
