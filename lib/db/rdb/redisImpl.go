package rdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisImpl 实现 IRedis 接口
type RedisImpl struct {
	// 可以保留一些必要的字段
	bizName string
}

// NewRedisImpl 创建一个新的 Redis 实现实例
func NewRedisImpl(bizName string) *RedisImpl {
	redisImpl := &RedisImpl{
		bizName: bizName,
	}

	return redisImpl
}

// getRedisClientByKey 根据 key 动态选择 Redis 实例
func (r *RedisImpl) getRedisClientByKey(ctx context.Context, key string) (*redis.Client, error) {
	if len(key) == 0 {
		return nil, errors.New("key 不能为空")
	}

	// 直接调用同包中的 GetRedisClient 函数
	client := GetRedisClient(ctx, r.bizName, key)

	if client == nil {
		return nil, errors.New("获取 Redis 客户端失败")
	}

	return client, nil
}

// Set 设置键值对
func (r *RedisImpl) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return err
	}

	if expiration > 0 {
		return client.Set(ctx, key, value, time.Duration(expiration)*time.Second).Err()
	} else {
		return client.Set(ctx, key, value, 0).Err()
	}
}

// Get 获取键值
func (r *RedisImpl) Get(ctx context.Context, key string) (string, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return "", err
	}

	return client.Get(ctx, key).Result()
}

// HMSet 设置哈希表值
func (r *RedisImpl) HMSet(ctx context.Context, key string, values map[string]interface{}) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return err
	}

	return client.HMSet(ctx, key, values).Err()
}

// HMGet 获取哈希表值
func (r *RedisImpl) HMGet(ctx context.Context, key string, fields ...string) (map[string]string, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	values, err := client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}

	for i, field := range fields {
		if i < len(values) {
			if values[i] != nil {
				result[field] = fmt.Sprintf("%v", values[i])
			} else {
				result[field] = ""
			}
		}
	}

	return result, nil
}

// Del 删除键
func (r *RedisImpl) Del(ctx context.Context, key string) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return err
	}

	return client.Del(ctx, key).Err()
}

// Exists 检查键是否存在
func (r *RedisImpl) Exists(ctx context.Context, key string) (bool, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return false, err
	}

	count, err := client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Expire 设置键的过期时间（秒）
func (r *RedisImpl) Expire(ctx context.Context, key string, expiration time.Duration) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return err
	}

	return client.Expire(ctx, key, time.Duration(expiration)*time.Second).Err()
}

// ZRangeByScore 获取有序集合中指定分数范围内的成员
func (r *RedisImpl) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return client.ZRangeByScore(ctx, key, opt).Result()
}
