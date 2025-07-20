package lib

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type RedisConfig struct {
	Addr     string // Redis服务器地址
	Password string // 密码（默认为空）
	DB       int    // 数据库编号
	PoolSize int    // 连接池大小
}

var rConfig RedisConfig

func RegisterRedisC(addr string, password string, db int, poolSize int) error {
	rConfig = RedisConfig{Addr: addr, Password: password, DB: db, PoolSize: poolSize}
	rdb := GetRedisClient()
	// 测试连接
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()

	if err != nil {
		fmt.Printf("redis connect fail%v\n", err)
		return NewXError(err, "连接Redis失败")
	}
	return NilXerr
}

func GetRedisClient() *redis.Client {
	// 创建Redis客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:     rConfig.Addr,
		Password: rConfig.Password,
		DB:       rConfig.DB,
		PoolSize: rConfig.PoolSize,
	})
	return rdb
}
