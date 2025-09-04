package rdb

import (
	"context"
	"fmt"
	"time"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"
	"github.com/ruandao/distribute-im-pkg/xetcd"

	"github.com/go-redis/redis/v8"
)

// RedisImpl 实现 IRedis 接口
type RedisImpl struct {
	// 可以保留一些必要的字段
	agent *RedisAgent
}

// NewRedisImpl 创建一个新的 Redis 实现实例
func NewRedisImpl(bizName string, xContent *xetcd.XContent, appConfig *appConfigLib.AppConfig) *RedisImpl {
	redisImpl := &RedisImpl{
		agent: NewRedisAgent(bizName, xContent, appConfig),
	}
	return redisImpl
}

func (r *RedisImpl) Close() {
	r.agent.Close()
}

func (r *RedisImpl) separateKey2ClientGroup(ctx context.Context, keys []string, forEachClientFunc KeysForClient) (map[string]ValErrItem, error) {
	if len(keys) == 0 {
		return nil, xerr.NewXError(fmt.Errorf("keys 不能为空"))
	}

	return r.agent.separateKey2AgentGroup(ctx, keys, forEachClientFunc)
}

// Set 设置键值对
func (r *RedisImpl) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.agent.Set(ctx, key, value, expiration)
}

// SetNX 只有当键不存在时才设置键值
func (r *RedisImpl) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
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
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return 0, err
	}

	ret, err := client.ZRem(ctx, key, members...).Result()
	return ret, xerr.NewXError(err)
}

// Rename 重命名键
func (r *RedisImpl) Rename(ctx context.Context, oldKey string, newKey string) error {
	// 使用旧键获取Redis客户端
	client, err := r.agent.getAgentByShareId(ctx, oldKey)
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
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return 0, err
	}

	ret, err := client.ZCount(ctx, key, min, max).Result()
	return ret, xerr.NewXError(err)
}

// Get 获取键值
func (r *RedisImpl) Get(ctx context.Context, key string) (string, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return "", err
	}

	ret, err := client.Get(ctx, key).Result()
	return ret, xerr.NewXError(err)
}

// HMSet 设置哈希表值
func (r *RedisImpl) HMSet(ctx context.Context, key string, values ...interface{}) error {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return err
	}

	return xerr.NewXError(client.HMSet(ctx, key, values...).Err())
}

// HMGet 获取哈希表值
func (r *RedisImpl) HMGet(ctx context.Context, key string, fields ...string) (map[string]string, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
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
			}
		}
	}

	return result, nil
}

// Del 删除键
func (r *RedisImpl) Del(ctx context.Context, key string) error {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return err
	}

	return xerr.NewXError(client.Del(ctx, key).Err())
}

// Exists 检查键是否存在
func (r *RedisImpl) Exists(ctx context.Context, key string) (bool, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
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
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return xerr.NewXError(err, "get redis client failed")
	}
	// 用于核对,过期时间是否设置正确
	cmd := client.Set(ctx, "updateKeyExpire"+key, expiration/time.Second, time.Hour)
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
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return nil, err
	}

	ret, err := client.ZRangeByScore(ctx, key, opt).Result()
	return ret, xerr.NewXError(err)
}

// ZAdd 向有序集合中添加成员
func (r *RedisImpl) ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return 0, err
	}

	ret, err := client.ZAdd(ctx, key, members...).Result()
	return ret, xerr.NewXError(err)
}

// HScan 迭代哈希表中的键值对
func (r *RedisImpl) HScan(ctx context.Context, key string, cursor uint64, match string, count int64) (uint64, map[string]string, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return 0, nil, err
	}

	// 调用 Redis 客户端的 HScan 方法
	cmd := client.HScan(ctx, key, cursor, match, count)

	// 获取扫描结果
	keysAndValues, nextCursor, err := cmd.Result()
	if err != nil {
		return 0, nil, xerr.NewXError(err)
	}

	// 解析返回的结果，转换为 map
	resultMap := make(map[string]string)
	for i := 1; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			resultMap[keysAndValues[i]] = keysAndValues[i+1]
		}
	}

	// 第一个返回值是下一个游标
	return nextCursor, resultMap, nil
}

type ValErrItem struct {
	Val any
	Err error
}

func (r *RedisImpl) MGet(ctx context.Context, keys ...string) (map[string]ValErrItem, error) {
	if len(keys) == 0 {
		return nil, xerr.NewXError(fmt.Errorf("keys 不能为空"))
	}

	// 使用第一个key获取Redis客户端
	clientM, err := r.separateKey2ClientGroup(ctx, keys, func(client *redis.Client, keyArr []string) map[string]ValErrItem {
		retM := make(map[string]ValErrItem)
		values, err := client.MGet(ctx, keyArr...).Result()
		for idx, key := range keyArr {
			retM[key] = ValErrItem{Val: values[idx], Err: err}
		}
		return retM
	})

	if err != nil {
		return nil, err
	}

	return clientM, nil
}

// MSet 批量设置多个键值对
func (r *RedisImpl) MSet(ctx context.Context, keyValM map[string]any) (map[string]ValErrItem, error) {
	keys := make([]string, 0, len(keyValM))
	for key := range keyValM {
		keys = append(keys, key)
	}

	retM, err := r.separateKey2ClientGroup(ctx, keys, func(client *redis.Client, keyArr []string) map[string]ValErrItem {
		values := make([]any, 0, 2*len(keyArr))
		for _, key := range keyArr {
			values = append(values, key)
			values = append(values, keyValM[key])
		}

		// 调用Redis客户端的MSet方法
		err := client.MSet(ctx, values...).Err()
		clientRetM := make(map[string]ValErrItem, len(keyArr))
		for _, key := range keyArr {
			clientRetM[key] = ValErrItem{Err: err}
		}
		return clientRetM
	})
	return retM, err
}

// ZRange 获取有序集合中指定索引范围内的成员
func (r *RedisImpl) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return nil, xerr.NewXError(err)
	}
	result, err := client.ZRange(ctx, key, start, stop).Result()
	return result, xerr.NewXError(err)
}

// ZRangeWithScores 获取有序集合中指定索引范围内的成员及其分数
func (r *RedisImpl) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return nil, xerr.NewXError(err)
	}
	result, err := client.ZRangeWithScores(ctx, key, start, stop).Result()
	return result, xerr.NewXError(err)
}

func (r *RedisImpl) HMDel(ctx context.Context, key string, fields ...string) (int64, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return 0, xerr.NewXError(err)
	}
	count, err := client.HDel(ctx, key, fields...).Result()
	return count, xerr.NewXError(err)
}

// 获取哈希表中所有字段和值
func (r *RedisImpl) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return nil, xerr.NewXError(err)
	}
	result, err := client.HGetAll(ctx, key).Result()
	return result, xerr.NewXError(err)
}

// 批量获取多个哈希表的字段数量
func (r *RedisImpl) MHCount(ctx context.Context, keys ...string) (map[string]ValErrItem, error) {
	if len(keys) == 0 {
		return nil, xerr.NewXError(fmt.Errorf("keys 不能为空"))
	}

	itemM, err := r.agent.separateKey2AgentGroup(ctx, keys, func(client *redis.Client, keyArr []string) (map[string]ValErrItem) {
		retM := make(map[string]ValErrItem)
		for _, key := range keyArr {
			intCmd := client.HLen(ctx, key)
			retM[key] = ValErrItem{Val: intCmd.Val(),Err: intCmd.Err()}
		}
		return retM
	})
	return itemM, err
}

// 移除键的过期时间，使其持久化
func (r *RedisImpl) Persist(ctx context.Context, key string) (bool, error) {
	client, err := r.agent.getAgentByShareId(ctx, key)
	if err != nil {
		return false, xerr.NewXError(err)
	}
	result, err := client.Persist(ctx, key).Result()
	return result, xerr.NewXError(err)
}

// 移除键的过期时间，使其持久化
func (r *RedisImpl) PersistX(ctx context.Context, key string) error {
	_, err := r.Persist(ctx, key)
	return err
}
