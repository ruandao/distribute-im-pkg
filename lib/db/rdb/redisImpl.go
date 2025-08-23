package rdb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/randx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"
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
						logx.Errorf("redis config unmarshal failed val: #%v# err: %v", conn, err)
						return
					}
					if err = PingTest(r._GetRedisClient(ctx, "", "", "", redisC)); err != nil {
						logx.Errorf("ping test to redis instance failed: %v", redisC)
						return
					}
				}
			}
		}

		if cluster == nil {
			logx.Errorf("redis biz route %v not found, err: %v", r.bizName, err)
		}
		r._cluster = cluster
	}
	r.watchRemoveFunc = r.xContent.ClusterWatch(r.bizName, cb)
	conns, err := r.xContent.GetDepServicesCluster(r.bizName)
	cb(conns, err)
	err = r.Set(ctx, "helloTest", "success", time.Hour)
	if err != nil {
		logx.Errorf("redis set %v with val: %v fail with err: %v", "helloTest", "success", err)
		return
	}
}

func (r *RedisImpl) Close() {
	if r.watchRemoveFunc != nil {
		logx.Errorf("call RedisImpl close")
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

	connStr := randx.SelectOne(instanceC.ConnConfig)
	redisC := &RedisConfig{}
	err = json.Unmarshal([]byte(connStr), redisC)
	if err != nil {
		return nil, xerr.NewXError(err, fmt.Sprintf("redis config parse failed val: #%v#", connStr))
	}
	return redisC, nil
}

func (r *RedisImpl) GetRedisClient(ctx context.Context, bizName string, key string) (*redis.Client, error) {
	routeTag := traffic.GetRouteTag(ctx)
	return r._GetRedisClient(ctx, bizName, routeTag, key, nil)
}

var dbMap sync.Map
var dbMapLocker sync.Mutex
func (r *RedisImpl) _GetRedisClient(ctx context.Context, bizName string, routeTag xetcd.RouteTag, key string, _redisC *RedisConfig) (rCli *redis.Client, err error) {
	// 创建Redis客户端
	// in default, the _redisC will be nil
	redisC := _redisC
	if redisC == nil {
		redisC, err = r.getRedisC(ctx, bizName, routeTag, key)
		if err != nil {
			return nil, xerr.NewXError(err)
		}
	}

	connStr := redisC.getDSN()
	val, ok := dbMap.Load(connStr)
	if !ok {
		dbMapLocker.Lock()
		defer dbMapLocker.Unlock()
		val, ok = dbMap.Load(connStr)
		if ok {
			return val.(*redis.Client), nil
		}
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisC.Addr,
			Password: redisC.Password,
			DB:       redisC.DB,
			PoolSize: redisC.PoolSize,
		})
		dbMap.Store(connStr, rdb)
		return rdb, nil

	}
	
	return val.(*redis.Client), nil
}

// getRedisClientByKey 根据 key 动态选择 Redis 实例
func (r *RedisImpl) getRedisClientByKey(ctx context.Context, key string) (*redis.Client, error) {
	if len(key) == 0 {
		return nil, xerr.NewXError(fmt.Errorf("key 不能为空"))
	}

	// 直接调用同包中的 GetRedisClient 函数
	client, err := r.GetRedisClient(ctx, r.bizName, key)
	if err != nil {
		return nil, err
	}

	if client == nil {
		return nil, xerr.NewXError(fmt.Errorf("获取 Redis 客户端失败"))
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
		return xerr.NewXError(client.Set(ctx, key, value, expiration).Err()) 
	} else {
		return xerr.NewXError(client.Set(ctx, key, value, 0).Err()) 
	}
}

// SetNX 只有当键不存在时才设置键值
func (r *RedisImpl) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return false, xerr.NewXError(err)
	}

	if expiration > 0 {
		ret, err := client.SetNX(ctx, key, value, expiration).Result()
		return ret, xerr.NewXError(err)
	} else {
		ret, err := client.SetNX(ctx, key, value, 0).Result()
		return ret, xerr.NewXError(err)
	}
}

// ZRem 从有序集合中删除一个或多个成员
func (r *RedisImpl) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return 0, err
	}

	ret, err := client.ZRem(ctx, key, members...).Result()
	return ret, xerr.NewXError(err)
}

// Rename 重命名键
func (r *RedisImpl) Rename(ctx context.Context, oldKey string, newKey string) error {
	// 使用旧键获取Redis客户端
	client, err := r.getRedisClientByKey(ctx, oldKey)
	if err != nil {
		return err
	}

	// 执行重命名操作
	err = client.Rename(ctx, oldKey, newKey).Err()
	if err != nil {
		return xerr.NewXError(err)
	}
	return nil
}

// ZCount 获取有序集合中指定分数范围内的成员数量
func (r *RedisImpl) ZCount(ctx context.Context, key string, min string, max string) (int64, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return 0, err
	}

	ret, err := client.ZCount(ctx, key, min, max).Result()
	return ret, xerr.NewXError(err)
}

// Get 获取键值
func (r *RedisImpl) Get(ctx context.Context, key string) (string, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return "", err
	}

	ret, err := client.Get(ctx, key).Result()
	return ret, xerr.NewXError(err)
}

// HMSet 设置哈希表值
func (r *RedisImpl) HMSet(ctx context.Context, key string, values ...interface{}) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return err
	}

	return xerr.NewXError(client.HMSet(ctx, key, values...).Err()) 
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
		return nil, xerr.NewXError(err, fmt.Sprintf("key: %v fields: %v values: %v err: %v", key, fields, values, err))
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

	return xerr.NewXError(client.Del(ctx, key).Err()) 
}

// Exists 检查键是否存在
func (r *RedisImpl) Exists(ctx context.Context, key string) (bool, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return false, err
	}

	count, err := client.Exists(ctx, key).Result()
	if err != nil {
		return false, xerr.NewXError(err)
	}

	return count > 0, nil
}

// Expire 设置键的过期时间（秒）
func (r *RedisImpl) Expire(ctx context.Context, key string, expiration time.Duration) error {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return xerr.NewXError(err, "get redis client failed")
	}
	// 用于核对,过期时间是否设置正确
	cmd := client.Set(ctx, "updateKeyExpire" + key, expiration/time.Second, time.Hour)
	if cmd.Err() != nil {
		logx.Errorf("updateKeyExpire err: %v", cmd.Err())
	}
	err = client.Expire(ctx, key, expiration).Err()
	if err != nil {
		return xerr.NewXError(err, fmt.Sprintf("setup Expire %v for key: %v failed, client: %v", expiration, key, client))
	}
	return nil
}

// ZRangeByScore 获取有序集合中指定分数范围内的成员
func (r *RedisImpl) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	ret, err := client.ZRangeByScore(ctx, key, opt).Result()
	return ret, xerr.NewXError(err)
}

// ZAdd 向有序集合中添加成员
func (r *RedisImpl) ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error) {
	client, err := r.getRedisClientByKey(ctx, key)
	if err != nil {
		return 0, err
	}

	ret, err := client.ZAdd(ctx, key, members...).Result()
	return ret, xerr.NewXError(err)
}
