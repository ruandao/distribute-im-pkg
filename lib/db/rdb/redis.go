package rdb

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type IRedis interface {
	// 根据key动态选择Redis实例并设置值
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	// 根据key动态选择Redis实例并获取值
	Get(ctx context.Context, key string) (string, error)
	// 根据key动态选择Redis实例并设置哈希表值
	HMSet(ctx context.Context, key string, values ...interface{}) error
	// 根据key动态选择Redis实例并获取哈希表值
	HMGet(ctx context.Context, key string, fields ...string) (map[string]string, error)
	// 根据key动态选择Redis实例并删除值
	Del(ctx context.Context, key string) error
	// 检查key是否存在
	Exists(ctx context.Context, key string) (bool, error)
	// 设置键的过期时间（秒）
	Expire(ctx context.Context, key string, expiration time.Duration) error
	// 获取有序集合中指定分数范围内的成员
	ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error)
	// 向有序集合中添加成员
	ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error)
	// 选择Redis实例的核心方法
	getRedisClientByKey(ctx context.Context, key string) (*redis.Client, error)
}
