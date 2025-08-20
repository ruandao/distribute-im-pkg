package rdb

import (
	"context"
	"fmt"

	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"github.com/go-redis/redis/v8"
)

type RedisConfig struct {
	Addr     string // Redis服务器地址
	Password string // 密码（默认为空）
	DB       int    // 数据库编号
	PoolSize int    // 连接池大小
}

func (redisC *RedisConfig)getDSN() string {
	return fmt.Sprintf("%v:%v:%v:%v", redisC.Addr, redisC.Password, redisC.DB, redisC.PoolSize)
}

var _redisImpl IRedis

func RegisterRedisC(redisImpl IRedis) {
	_redisImpl = redisImpl
}
func GetRedisC() IRedis {
	if _redisImpl == nil {
		panic("_redisImpl shouldn't be nil")
	}
	return _redisImpl
}

func PingTest(rdb *redis.Client, err error) error {
	if err != nil {
		return err
	}
	// rdb := redisC.GetRedisClient("default", "")
	// 测试连接
	ctx := context.Background()
	_, err = rdb.Ping(ctx).Result()

	if err != nil {
		fmt.Printf("redis connect fail%v\n", err)
		return xerr.NewXError(err, "连接Redis失败")
	}
	return xerr.NilXerr
}
