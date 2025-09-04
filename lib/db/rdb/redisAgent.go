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
	"github.com/ruandao/distribute-im-pkg/lib/runner"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"
	"github.com/ruandao/distribute-im-pkg/traffic"
	"github.com/ruandao/distribute-im-pkg/xetcd"

	"github.com/go-redis/redis/v8"
)

// RedisImpl 实现 IRedis 接口
type RedisAgent struct {
	// 可以保留一些必要的字段
	bizName         string
	xContent        *xetcd.XContent
	_cluster        *xetcd.RouteShareConns
	watchRemoveFunc func()
	appConfig       *appConfigLib.AppConfig
}

// NewRedisImpl 创建一个新的 Redis 实现实例
func NewRedisAgent(bizName string, xContent *xetcd.XContent, appConfig *appConfigLib.AppConfig) *RedisAgent {
	redisAgent := &RedisAgent{
		bizName:   bizName,
		xContent:  xContent,
		appConfig: appConfig,
	}
	redisAgent.Watch()
	return redisAgent
}

func (r *RedisAgent) Watch() {
	ctx := context.Background()
	var cb xetcd.ClusterChangeCB = func(cluster *xetcd.RouteShareConns, err error) {
		if err != nil {
			logx.Errorf("redis biz route %v not found, err: %v", r.bizName, err)
			r._cluster = nil
			return
		}
		for _, shareClusters := range cluster.M {
			for _, shareConns := range shareClusters {
				shareConns.ConnConfig.Range(func(key, value any) bool {
					ipport := key.(string)
					conn := value.(string)
					redisC := &RedisConfig{}
					err := json.Unmarshal([]byte(conn), redisC)
					if err != nil {
						logx.ErrorX("redis config unmarshal failed ")(
							fmt.Sprintf("ipport: %v ", ipport),
							fmt.Sprintf("val: %v ", conn),
							fmt.Sprintf("err: %v ", err),
						)
						return true
					}
					if err = PingTest(r._GetAgent(ctx, "", "", "", redisC)); err != nil {
						logx.ErrorX("ping test to redis instance failed")(
							fmt.Sprintf("config: %v", redisC),
						)
						return true
					}
					return true
				})

			}
		}

		if cluster == nil {
			logx.Errorf("redis biz route %v not found, err: %v", r.bizName, err)
		}
		r._cluster = cluster
	}
	r.watchRemoveFunc = r.xContent.ClusterWatch(ctx, r.bizName, cb)
	conns, err := r.xContent.GetDepServicesCluster(r.bizName)
	cb(conns, err)
	err = r.Set(ctx, "helloTest", "success", time.Hour)
	if err != nil {
		logx.Errorf("redis set %v with val: %v fail with err: %v", "helloTest", "success", err)
		return
	}
}

func (r *RedisAgent) Close() {
	if r.watchRemoveFunc != nil {
		logx.Errorf("call RedisAgent close")
		r.watchRemoveFunc()
		r.watchRemoveFunc = nil
	}
}

func (r *RedisAgent) getAgentC(ctx context.Context, bizName string, routeTag xetcd.RouteTag, key string) (*RedisConfig, error) {
	shareKey, err := r.appConfig.GetShareKeyFromId(ctx, bizName, key)
	if err != nil {
		return nil, err
	}
	instanceC, err := r._cluster.Get(shareKey, routeTag)
	if err != nil {
		return nil, err
	}

	connKey := randx.SelectOne(instanceC.ConnConfig.Keys())
	connStr, _ := instanceC.ConnConfig.Load(connKey)
	redisC := &RedisConfig{}
	err = json.Unmarshal([]byte(connStr.(string)), redisC)
	if err != nil {
		return nil, xerr.NewXError(err, fmt.Sprintf("redis config parse failed val: #%v#", connStr))
	}
	return redisC, nil
}

func (r *RedisAgent) GetAgent(ctx context.Context, bizName string, key string) (*redis.Client, error) {
	routeTag := traffic.GetRouteTag(ctx)
	return r._GetAgent(ctx, bizName, routeTag, key, nil)
}

var dbMap sync.Map
var dbMapLocker sync.Mutex

func (r *RedisAgent) _GetAgent(ctx context.Context, bizName string, routeTag xetcd.RouteTag, key string, _redisC *RedisConfig) (rCli *redis.Client, err error) {
	// 创建Redis客户端
	// in default, the _redisC will be nil
	redisC := _redisC
	if redisC == nil {
		redisC, err = r.getAgentC(ctx, bizName, routeTag, key)
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

type KeysForClient func(client *redis.Client, keyArr []string) map[string]ValErrItem

func (r *RedisAgent) separateKey2AgentGroup(ctx context.Context, keys []string, forEachClientFunc KeysForClient) (map[string]ValErrItem, error) {
	if len(keys) == 0 {
		return nil, xerr.NewXError(fmt.Errorf("keys 不能为空"))
	}

	// 将 key 按分片分组，然后同一组的键一起处理
	retMCh := runner.MapReduce(ctx, "separateKey2AgentGroup", runner.Arr2Ch(keys), func(key string) string {
		shareKey, _ := r.appConfig.GetShareKeyFromId(ctx, r.bizName, key)
		return string(shareKey)
	}, 10, func(shareKey string, keyArr []string) map[string]ValErrItem {
		tmpM := make(map[string]ValErrItem)
		client, err := r.getAgentByShareId(ctx, shareKey)
		if err != nil {
			for _, key := range keyArr {
				tmpM[key] = ValErrItem{Val: nil, Err: err}
			}
			return tmpM
		}
		keyValErr := forEachClientFunc(client, keyArr)
		return keyValErr
	})

	retM := make(map[string]ValErrItem)
	for itemM := range retMCh {
		for key, val := range itemM {
			retM[key] = val
		}
	}
	return retM, nil
}

type CmdForClient func(client *redis.Client) map[string]ValErrItem

func (r *RedisAgent) allClientGroup(ctx context.Context, forEachClientFunc CmdForClient) (map[string]ValErrItem, error) {
	clients, err := r._getAllAgent(ctx)
	if err != nil {
		return nil, err
	}

	retM := make(map[string]ValErrItem)

	ch := make(chan map[string]ValErrItem, len(clients))
	defer close(ch)

	taskWG := sync.WaitGroup{}
	resultWg := sync.WaitGroup{}

	for _, client := range clients {
		taskWG.Add(1)
		go func(_client *redis.Client) {
			defer taskWG.Done()
			itemResult := forEachClientFunc(_client)
			ch <- itemResult
		}(client)
	}

	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		// 收集结果

		for item := range ch {
			for k, v := range item {
				retM[k] = v
			}
		}
	}()

	taskWG.Wait()
	close(ch)
	resultWg.Wait()

	return retM, nil
}

func (r *RedisAgent) _getAllAgent(ctx context.Context) ([]*redis.Client, error) {
	// shares, err := appConfigLib.GetAppConfig().GetShares(ctx, r.bizName)
	// if err != nil {
	// 	return nil, err
	// }

	// shareKeys :=
	// todo
	return nil, nil
}

// getRedisClientByKey 根据 key 动态选择 Redis 实例
func (r *RedisAgent) getAgentByShareId(ctx context.Context, key string) (*redis.Client, error) {
	if len(key) == 0 {
		return nil, xerr.NewXError(fmt.Errorf("key 不能为空"))
	}

	// 直接调用同包中的 GetRedisClient 函数
	client, err := r.GetAgent(ctx, r.bizName, key)
	if err != nil {
		return nil, err
	}

	if client == nil {
		return nil, xerr.NewXError(fmt.Errorf("获取 Redis 客户端失败"))
	}

	return client, nil
}

// Set 设置键值对
func (r *RedisAgent) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	client, err := r.getAgentByShareId(ctx, key)
	if err != nil {
		return err
	}

	if expiration > 0 {
		return xerr.NewXError(client.Set(ctx, key, value, expiration).Err())
	} else {
		return xerr.NewXError(client.Set(ctx, key, value, 0).Err())
	}
}
