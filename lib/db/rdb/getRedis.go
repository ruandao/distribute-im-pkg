package rdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"github.com/ruandao/distribute-im-pkg/xetcd"

	"github.com/ruandao/distribute-im-pkg/traffic"

	"github.com/go-redis/redis/v8"
)

type RedisConfig struct {
	Addr     string // Redis服务器地址
	Password string // 密码（默认为空）
	DB       int    // 数据库编号
	PoolSize int    // 连接池大小
}

var _cluster *xetcd.RouteShareConns

func RegisterRedisC(bizName string, xContent *xetcd.XContent) {
	ctx := context.Background()
	var cb xetcd.ClusterChangeCB = func(cluster *xetcd.RouteShareConns) {
		for _, shareClusters := range cluster.M {
			for _, shareConns := range shareClusters {
				for _, conn := range shareConns.ConnConfig {
					redisC := &RedisConfig{}
					err := json.Unmarshal([]byte(conn), redisC)
					if err != nil {
						logx.Errorf("redis config unmarshal failed: %v", err)
						return
					}
					if err = PingTest(_GetRedisClient(ctx, "", "", "", redisC)); err != nil {
						logx.Errorf("ping test to redis instance failed: %v", redisC)
						return
					}
				}
			}
		}

		_cluster = cluster
	}
	xContent.ClusterWatch(bizName, cb)
}

func Close() {
	// todo
	_cluster = nil
}

func getRedisC(ctx context.Context, bizName string, routeTag string, key string) *RedisConfig {
	shareKey := appConfigLib.GetAppConfig().GetShareKeyFromId(ctx, bizName, key)
	instanceC := _cluster.Get(routeTag, shareKey)
	redisC := &RedisConfig{}
	json.Unmarshal([]byte(instanceC.ConnConfig[0]), redisC)
	return redisC
}

func GetRedisClient(ctx context.Context, bizName string, key string) *redis.Client {
	routeTag := traffic.GetRouteTag(ctx)
	return _GetRedisClient(ctx, bizName, routeTag, key, nil)
}

func _GetRedisClient(ctx context.Context, bizName string, routeTag string, key string, _redisC *RedisConfig) *redis.Client {
	// 创建Redis客户端
	// in default, the _redisC will be nil
	redisC := _redisC
	if redisC == nil {
		redisC = getRedisC(ctx, bizName, routeTag, key)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisC.Addr,
		Password: redisC.Password,
		DB:       redisC.DB,
		PoolSize: redisC.PoolSize,
	})
	return rdb
}

func PingTest(rdb *redis.Client) error {
	// rdb := redisC.GetRedisClient("default", "")
	// 测试连接
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()

	if err != nil {
		fmt.Printf("redis connect fail%v\n", err)
		return xerr.NewXError(err, "连接Redis失败")
	}
	return xerr.NilXerr
}
