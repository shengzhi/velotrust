package cache

import (
	"context"
	"strings"
	"time"
)

// Cache 统一缓存接口
type Cache interface {
	// Get 获取缓存，并将 JSON 结果反序列化到 dest 指针对象中
	Get(ctx context.Context, key string, dest interface{}) (bool, error)
	// Set 设置缓存，自动处理对象的序列化
	Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error
	// Delete 删除缓存
	Delete(ctx context.Context, key string) error

	Exist(ctx context.Context, key string) (bool, error)
}

// GlobalCache 全局缓存句柄，供 Service 层直接调用
var GlobalCache Cache

func NewKey(args ...string) string {
	return strings.Join(args, "")
}
