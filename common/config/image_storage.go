package config

// Image storage: where generated images are uploaded so a request that asks for a URL gets one
// (common/storage). Set in the platform settings (网关 → 请求与协议 → 图片存储); an empty
// provider keeps it off and those responses carry base64 instead.
var (
	ImageStorageProvider = ""

	ImageStorageS3Endpoint        = ""
	ImageStorageS3Region          = ""
	ImageStorageS3AccessKeyId     = ""
	ImageStorageS3AccessKeySecret = ""
	ImageStorageS3BucketName      = ""
	ImageStorageS3CDNURL          = ""
	ImageStorageS3ExpirationDays  = 0

	ImageStorageOSSEndpoint        = ""
	ImageStorageOSSRegion          = ""
	ImageStorageOSSAccessKeyId     = ""
	ImageStorageOSSAccessKeySecret = ""
	ImageStorageOSSBucketName      = ""

	ImageStorageSMMSSecret    = ""
	ImageStorageImgurClientId = ""
)
