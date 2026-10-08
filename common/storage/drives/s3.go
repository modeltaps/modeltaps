package drives

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Upload struct {
	EndPoint        string
	CustomDomain    string
	AccessKeyId     string
	AccessKeySecret string
	BucketName      string
	expirationDays  int
}

func NewS3Upload(endpoint, accessKeyId, accessKeySecret, bucketName, cdnurl string, expirationDays int) *S3Upload {
	_cdnurl := cdnurl
	if _cdnurl == "" {
		_cdnurl = endpoint
	}
	return &S3Upload{
		EndPoint:        endpoint,
		BucketName:      bucketName,
		CustomDomain:    _cdnurl,
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		expirationDays:  expirationDays,
	}
}

func (a *S3Upload) Name() string {
	return "S3"
}

func (a *S3Upload) Upload(data []byte, s3Key string) (string, error) {

	// SDK v2 的 BaseEndpoint 必须带 scheme；v1 时代配置的裸主机名默认按 https 处理
	endpoint := a.EndPoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	svc := s3.New(s3.Options{
		Credentials:  credentials.NewStaticCredentialsProvider(a.AccessKeyId, a.AccessKeySecret, ""),
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		UsePathStyle: true,
		// 只在接口要求时才算校验和：默认的 CRC 头会被部分 S3 兼容存储拒绝
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	ctx := context.Background()

	// 获取当前日期作为文件名前缀
	now := time.Now()
	datePrefix := fmt.Sprintf("%d-%02d-%02d/", now.Year(), now.Month(), now.Day())

	// 将日期前缀添加到文件名
	datedKey := datePrefix + s3Key

	// 检查文件是否已存在于 S3
	_, err := svc.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(a.BucketName),
		Key:    aws.String(datedKey),
	})

	if err == nil {
		// 文件已存在，直接返回自定义域名 URL
		return fmt.Sprintf("%s/%s", a.CustomDomain, datedKey), nil
	}
	fileBytes := bytes.NewReader(data)

	// 准备上传参数
	putObjectInput := &s3.PutObjectInput{
		Bucket: aws.String(a.BucketName),
		Key:    aws.String(datedKey),
		Body:   fileBytes,
	}

	// 如果设置了过期时间，则添加过期策略
	if a.expirationDays > 0 {
		// 计算过期时间
		expirationDate := now.AddDate(0, 0, a.expirationDays)
		putObjectInput.Expires = aws.Time(expirationDate)
	}

	// 上传文件到 S3
	_, err = svc.PutObject(ctx, putObjectInput)

	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %v", err)
	}

	return fmt.Sprintf("%s/%s", a.CustomDomain, datedKey), nil
}
