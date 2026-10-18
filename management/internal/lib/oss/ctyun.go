package oss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// CTYunOSS is a CTYun OBS client backed by AWS SDK for Go v2's S3 client.
// The public API is kept compatible with the previous implementation.
type CTYunOSS struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	initErr       error
}

var _ OSS = (*CTYunOSS)(nil)

func NewCTYunOSS(accessKey, secretKey, endpoint, region string) *CTYunOSS {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return &CTYunOSS{initErr: fmt.Errorf("oss: endpoint is required")}
	}
	if _, err := parseEndpoint(endpoint); err != nil {
		return &CTYunOSS{initErr: err}
	}
	if strings.TrimSpace(region) == "" {
		region = "cn-jssz1"
	}

	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithHTTPClient(&http.Client{Transport: &http.Transport{MaxIdleConns: 5}, Timeout: 120 * time.Second}),
	)
	if err != nil {
		return &CTYunOSS{initErr: fmt.Errorf("oss: load AWS SDK v2 config: %w", err)}
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	return &CTYunOSS{
		client:        client,
		presignClient: s3.NewPresignClient(client),
	}
}

func (c *CTYunOSS) ProviderName() string { return "CTYunOSS" }

// PutObject uploads an object. Single-request upload is limited by the
// provider's object-size limit, as in the previous implementation.
func (c *CTYunOSS) PutObject(bucket, key string, data []byte) error {
	return c.Put(context.Background(), bucket, key, bytes.NewReader(data))
}

func (c *CTYunOSS) Put(ctx context.Context, bucket, key string, body io.Reader) error {
	if err := c.ready(); err != nil {
		return err
	}
	if err := validateBucketKey(bucket, key); err != nil {
		return err
	}
	if body == nil {
		return fmt.Errorf("oss: object data is required")
	}
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("put object %s/%s: %w", bucket, key, err)
	}
	return nil
}

// GetObject retrieves an object. The returned body must be closed by the caller.
func (c *CTYunOSS) GetObject(bucket, key string) (io.ReadCloser, error) {
	return c.Get(context.Background(), bucket, key)
}

func (c *CTYunOSS) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	if err := validateBucketKey(bucket, key); err != nil {
		return nil, err
	}
	output, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s/%s: %w", bucket, key, err)
	}
	return output.Body, nil
}

func (c *CTYunOSS) GetDownloadURL(bucket, key string, expire time.Duration) (string, error) {
	return c.GetInlineDownloadURL(bucket, key, "", expire)
}

// GetInlineDownloadURL overrides the response headers so browsers render the
// object in an embedded viewer instead of treating it as an attachment.
func (c *CTYunOSS) GetInlineDownloadURL(bucket, key, contentType string, expire time.Duration) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	if err := validateBucketKey(bucket, key); err != nil {
		return "", err
	}
	if expire <= 0 {
		return "", fmt.Errorf("oss: expire must be positive")
	}
	output, err := c.presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket:                     aws.String(bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("inline"),
		ResponseContentType:        optionalString(contentType),
	}, func(options *s3.PresignOptions) {
		options.Expires = expire
	})
	if err != nil {
		return "", fmt.Errorf("presign download %s/%s: %w", bucket, key, err)
	}
	return output.URL, nil
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return aws.String(strings.TrimSpace(value))
}

func (c *CTYunOSS) DeleteObjects(bucket string, keys ...string) error {
	return c.Delete(context.Background(), bucket, keys...)
}

func (c *CTYunOSS) Delete(ctx context.Context, bucket string, keys ...string) error {
	if err := c.ready(); err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("oss: bucket is required")
	}
	objects := make([]s3types.ObjectIdentifier, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("oss: key is required")
		}
		objects = append(objects, s3types.ObjectIdentifier{Key: aws.String(key)})
	}
	if len(objects) == 0 {
		return nil
	}
	output, err := c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{Objects: objects, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return fmt.Errorf("delete objects in %s: %w", bucket, err)
	}
	for _, deletionError := range output.Errors {
		return fmt.Errorf("delete object %s/%s: %s", bucket, aws.ToString(deletionError.Key), aws.ToString(deletionError.Message))
	}
	return nil
}

func (c *CTYunOSS) Exist(bucket, key string) bool {
	ok, _ := c.Exists(context.Background(), bucket, key)
	return ok
}

func (c *CTYunOSS) Exists(ctx context.Context, bucket, key string) (bool, error) {
	if err := c.ready(); err != nil {
		return false, err
	}
	if err := validateBucketKey(bucket, key); err != nil {
		return false, err
	}
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil, err
}

func (c *CTYunOSS) PublicURL(bucket, fileURL string, expire time.Duration) (string, error) {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(fileURL) == "" {
		return "", nil
	}
	return c.GetDownloadURL(bucket, fileURL, expire)
}

func (c *CTYunOSS) UploadURL(bucket, fileURL string, expire time.Duration) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	if err := validateBucketKey(bucket, fileURL); err != nil {
		return "", err
	}
	if expire <= 0 {
		return "", fmt.Errorf("oss: expire must be positive")
	}
	output, err := c.presignClient.PresignPutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fileURL),
	}, func(options *s3.PresignOptions) {
		options.Expires = expire
	})
	if err != nil {
		return "", fmt.Errorf("presign upload %s/%s: %w", bucket, fileURL, err)
	}
	return output.URL, nil
}

func (c *CTYunOSS) ready() error {
	if c == nil {
		return fmt.Errorf("oss: client is nil")
	}
	if c.initErr != nil {
		return c.initErr
	}
	if c.client == nil || c.presignClient == nil {
		return fmt.Errorf("oss: client is not initialized")
	}
	return nil
}

func validateBucketKey(bucket, key string) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("oss: bucket is required")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("oss: key is required")
	}
	return nil
}

func parseEndpoint(endpoint string) (*url.URL, error) {
	raw := strings.TrimSpace(endpoint)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("oss: invalid endpoint")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, fmt.Errorf("oss: endpoint scheme must be http or https")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("oss: endpoint path is not supported")
	}
	return parsed, nil
}
