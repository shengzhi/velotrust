package oss

import (
	"context"
	"io"
	"time"
)

// OSS defines the object-storage capabilities consumed by application code.
// Operations are context-aware and stream object data to avoid full-object
// buffering; CTYunOSS and other providers are interchangeable implementations.
type OSS interface {
	ProviderName() string
	PutObject(bucket, key string, data []byte) error
	GetObject(bucket, key string) (io.ReadCloser, error)
	GetDownloadURL(bucket, key string, expire time.Duration) (string, error)
	GetInlineDownloadURL(bucket, key, contentType string, expire time.Duration) (string, error)
	DeleteObjects(bucket string, keys ...string) error
	Exist(bucket, key string) bool
	PublicURL(bucket, key string, expire time.Duration) (string, error)
	UploadURL(bucket, key string, expire time.Duration) (string, error)

	Put(context.Context, string, string, io.Reader) error
	Get(context.Context, string, string) (io.ReadCloser, error)
	Delete(context.Context, string, ...string) error
	Exists(context.Context, string, string) (bool, error)
}
