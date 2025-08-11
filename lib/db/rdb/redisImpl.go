package rdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/traffic"
	"github.com/ruandao/distribute-im-pkg/xetcd"

	"github.com/go-redis/redis/v8"
)

// RedisImpl 实现 IRedis 接口
type RedisImpl struct {
	// 可以保留一些必要的字段
	bizName         string
	xContent        *xetcd.XContent
	_cluster        *xetcd.RouteShareConns
	watchRemoveFunc func()
	appConfig       *appConfigLib.AppConfig
}

// NewRedisImpl 创建一个新的 Redis 实现实例
func NewRedisImpl(bizName string, xContent *xetcd.XContent, appConfig *appConfigLib.AppConfig) *RedisImpl {
	redisImpl := &RedisImpl{
		bizName:   bizName,
		xContent:  xContent,
		appConfig: appConfig,
	}
	redisImpl.Watch()
	return redisImpl
}

func (r *RedisImpl) Watch() {
	ctx := context.Background()
	var cb xetcd.ClusterChangeCB = func(cluster *xetcd.RouteShareConns, err error) {
		if err != nil {
			logx.Errorf("redis biz route %v not found, err: %v", r.bizName, err)
			r._cluster = nil
			return
		}
		for _, shareClusters := range cluster.M {
			for _, shareConns := range shareClusters {
				for _, conn := range shareConns.ConnConfig {
					redisC := &RedisConfig{}
					err := json.Unmarshal([]byte(conn), redisC)
					if err != nil {
						logx.Errorf("redis config unmarshal failed: %v", err)
						return
					}
					if err = PingTest(r._GetRedisClient(ctx, "", "", "", redisC)); err != nil {
						logx.Errorf("ping test to redis instance failed: %v", redisC)
						return
					}
				}
			}
		}

		r._cluster = cluster
	}
	r.watchRemoveFunc = r.xContent.ClusterWatch(r.bizName, cb)
}

func (r *RedisImpl) Close() {
	if r.watchRemoveFunc != nil {
		r.watchRemoveFunc()
		r.watchRemoveFunc = nil
	}
}

func (r *RedisImpl) getRedisC(ctx context.Context, bizName string, routeTag xetcd.RouteTag, key string) (*RedisConfig, error) {
	shareKey, err := r.appConfig.GetShareKeyFromId(ctx, bizName, key)
	if err != nil {
		return nil, err
	}
	instanceC, err := r._cluster.Get(shareKey, routeTag)
	if err != nil {
		return nil, err
	}
	redisC := &RedisConfig{}
	json.Unmarshal([]byte(instanceC.ConnConfig[0]), redisC)
	return redisC, nil
}

func (r *RedisImpl) GetRedisClient(ctx context.Context, bizName string, key string) (*redis.Client, error) {
	routeTag := traffic.GetRouteTag(ctx)
	return r._GetRedisClient(ctx, bizName, routeTag, key, nil)
}

func (r *RedisImpl) _GetRedisClient(ctx context.Context, bizName string, routeTag xetcd.RouteTag, key string, _redisC *RedisConfig) (rCli *redis.Client, err error) {
	// 创建Redis客户端
	// in default, the _redisC will be nil
	redisC := _redisC
	if redisC == nil {
		redisC, err = r.getRedisC(ctx, bizName, routeTag, key)
		if err != nil {
			return nil, err
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisC.Addr,
		Password: redisC.Password,
		DB:       redisC.DB,
		PoolSize: redisC.PoolSize,
	})
	return rdb, nil
}

// getRedisClientByKey 根据 key 动态选择 Redis 实例
func (r *RedisImpl) getRedisClientByKey(ctx context.Context, key string) (*redis.Client, error) {
	if len(key) == 0 {
		return nil, errors.New("key 不能为空")
	}

	// 直接调用同包中的 GetRedisClient 函数
	client, err := r.GetRedisClient(ctx, r.bizName, key)
	if err != nil {
		return nil, err
	}

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
func (r *RedisImpl) HMSet(ctx context.Context, key string, values ...interface{}) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return err
	}

	return client.HMSet(ctx, key, values...).Err()
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

// ZAdd 向有序集合中添加成员
func (r *RedisImpl) ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return 0, err
	}

	return client.ZAdd(ctx, key, members...).Result()
}
