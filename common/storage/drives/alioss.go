package drives

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// AliOSSUpload uploads to Alibaba Cloud OSS with V4 signatures. Accounts created from March 2025
// and buckets created from September 2025 reject V1 signatures, and V4 needs the bucket region.
type AliOSSUpload struct {
	Endpoint        string
	Region          string
	AccessKeyId     string
	AccessKeySecret string
	BucketName      string
}

// NewAliOSSUpload builds the driver. An empty region is derived from a standard endpoint such as
// oss-cn-beijing.aliyuncs.com; Upload fails when it can be neither set nor derived.
func NewAliOSSUpload(endpoint, region, accessKeyId, accessKeySecret, bucketName string) *AliOSSUpload {
	region = strings.TrimSpace(region)
	if region == "" {
		region = AliOSSRegionFromEndpoint(endpoint)
	}
	return &AliOSSUpload{
		Endpoint:        endpoint,
		Region:          region,
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		BucketName:      bucketName,
	}
}

// AliOSSRegionFromEndpoint returns the region of a standard OSS endpoint
// (oss-<region>.aliyuncs.com or oss-<region>-internal.aliyuncs.com, with or without a scheme),
// or "" for anything else, such as a custom domain or the global acceleration endpoint.
func AliOSSRegionFromEndpoint(endpoint string) string {
	host := strings.TrimSpace(endpoint)
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		host = u.Hostname()
	} else if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)

	label, rest, ok := strings.Cut(host, ".")
	if !ok || rest != "aliyuncs.com" || !strings.HasPrefix(label, "oss-") {
		return ""
	}
	region := strings.TrimSuffix(strings.TrimPrefix(label, "oss-"), "-internal")
	switch region {
	case "", "accelerate", "accelerate-overseas":
		return ""
	}
	return region
}

func (a *AliOSSUpload) Name() string {
	return "AliOSS"
}

func (a *AliOSSUpload) Upload(data []byte, fileName string) (string, error) {
	if a.Region == "" {
		return "", errors.New("OSS region is not set and cannot be derived from the endpoint; set storage.alioss.region, for example cn-beijing")
	}

	client, err := oss.New(a.Endpoint, a.AccessKeyId, a.AccessKeySecret,
		oss.AuthVersion(oss.AuthV4), oss.Region(a.Region))
	if err != nil {
		return "", fmt.Errorf("creating OSS client: %w", err)
	}

	bucket, err := client.Bucket(a.BucketName)
	if err != nil {
		return "", fmt.Errorf("getting bucket: %w", err)
	}

	if err := bucket.PutObject(fileName, bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("uploading file: %w", err)
	}

	objectURL, err := bucket.SignURL(fileName, oss.HTTPGet, 3600)
	if err != nil {
		return "", fmt.Errorf("signing object URL: %w", err)
	}

	return objectURL, nil
}
