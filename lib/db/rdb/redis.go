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
	// 根据key动态选择Redis实例并删除哈希表字段
	HMDel(ctx context.Context, key string, fields ...string) (int64, error)
	// 根据key动态选择Redis实例获取哈希表所有字段和值
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	// 批量获取多个哈希表的字段数量
	MHCount(ctx context.Context, keys ...string) (map[string]ValErrItem, error)
	// 根据key动态选择Redis实例并删除值
	Del(ctx context.Context, key string) error
	// 检查key是否存在
	Exists(ctx context.Context, key string) (bool, error)
	// 设置键的过期时间（秒）
	Expire(ctx context.Context, key string, expiration time.Duration) error
	// 移除键的过期时间，使其持久化
	Persist(ctx context.Context, key string) (bool, error)
	PersistX(ctx context.Context, key string) (error)
	// 获取有序集合中指定分数范围内的成员
	ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error)
	// 向有序集合中添加成员
	ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error)
	// 只有当键不存在时才设置键值
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	// 从有序集合中删除一个或多个成员
	ZRem(ctx context.Context, key string, members ...interface{}) (int64, error)
	// 重命名键
	Rename(ctx context.Context, oldKey string, newKey string) error
	// 获取有序集合中指定分数范围内的成员数量
	ZCount(ctx context.Context, key string, min string, max string) (int64, error)
	// 迭代哈希表中的键值对
	HScan(ctx context.Context, key string, cursor uint64, match string, count int64) (uint64, map[string]string, error)
	// MGET 批量获取多个键的值
	MGet(ctx context.Context, keys ...string) (map[string]ValErrItem, error)
	// MSET 批量设置多个键值对
	MSet(ctx context.Context, keyValM map[string]any) (map[string]ValErrItem, error)
	// 获取有序集合中指定索引范围内的成员
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	// 获取有序集合中指定索引范围内的成员及其分数
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error)
}
