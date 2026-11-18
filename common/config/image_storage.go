package config

// Image storage: where generated images are uploaded so a request that asks for a URL gets one
// (common/storage). Set in the platform settings (网关 → 请求与协议 → 图片存储). The default,
// "local", keeps them on the gateway for an hour; "off" answers with base64 instead.
var (
	ImageStorageProvider = "local"

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

// ImageStorageRehostUpstream also moves images an upstream returns as links (DALL·E, Flux,
// Seedream ...) into the image storage, so clients get a link on the site's own address that
// works where the upstream host is blocked or slow. A link that cannot be fetched is passed on.
var ImageStorageRehostUpstream = true
