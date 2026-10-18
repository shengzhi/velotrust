package oss

import (
	"context"
	"io"
	"time"
)

// compileOnlyProvider proves callers can substitute any OSS implementation.
type compileOnlyProvider struct{}

func (compileOnlyProvider) ProviderName() string                            { return "test" }
func (compileOnlyProvider) PutObject(string, string, []byte) error          { return nil }
func (compileOnlyProvider) GetObject(string, string) (io.ReadCloser, error) { return nil, nil }
func (compileOnlyProvider) GetDownloadURL(string, string, time.Duration) (string, error) {
	return "", nil
}
func (compileOnlyProvider) GetInlineDownloadURL(string, string, string, time.Duration) (string, error) {
	return "", nil
}
func (compileOnlyProvider) DeleteObjects(string, ...string) error                   { return nil }
func (compileOnlyProvider) Exist(string, string) bool                               { return false }
func (compileOnlyProvider) PublicURL(string, string, time.Duration) (string, error) { return "", nil }
func (compileOnlyProvider) UploadURL(string, string, time.Duration) (string, error) { return "", nil }
func (compileOnlyProvider) Put(context.Context, string, string, io.Reader) error    { return nil }
func (compileOnlyProvider) Get(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}
func (compileOnlyProvider) Delete(context.Context, string, ...string) error      { return nil }
func (compileOnlyProvider) Exists(context.Context, string, string) (bool, error) { return false, nil }

var _ OSS = compileOnlyProvider{}
