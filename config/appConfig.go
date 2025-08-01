package config

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sortdeplist "github.com/ruandao/distribute-im-pkg/config/util/sortDepList"
	lib "github.com/ruandao/distribute-im-pkg/lib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	etcdLib "go.etcd.io/etcd/client/v3"
)

type DBConfig struct {
	User     string `mapstruct:"user"`
	Password string `mapstruct:"password"`
	Addr     string `mapstruct:"addr"`
	DBName   string `mapstruct:"dbName"`
}

type RedisConfig struct {
	Addr     string `mapstruct:"addr"`     // Redis服务器地址
	Password string `mapstruct:"password"` // 密码（默认为空）
	DB       int    `mapstruct:"db"`       // 数据库编号
	PoolSize int    `mapstruct:"poolSize"` // 连接池大小
}

type AppConfig struct {
	TrafficTags []string `mapstructure:"trafficTags"`
	// 为什么需要从配置中读取？程序是知道自己依赖哪些服务的，但是编排程序的人不知道，所以需要强制二者一致，这样部署维护就更容易
	DepServices []string    `mapstructure:"depServices"`
	DBConfig    DBConfig    `mapstructure:"dbConfig"`
	RedisConfig RedisConfig `mapstructure:"redisConfig"`
}

var appConfCh chan AppConfig
var once sync.Once
var depListVal atomic.Value

func RegisterDepList(depList []string) {
	depListVal.Store(depList)
	fmt.Printf("depList: %v load: %v", depList, depListVal.Load())
	once.Do(func() {
		depList := depListVal.Load().([]string)
		sortdeplist.Sort(depList)

		appConfCh = make(chan AppConfig)
		writeAppConf(AppConfig{})

		go func() {
			for {
				appConf := <-appConfCh
				writeAppConf(appConf)
			}
		}()
	})
}

func readAppConfig(ctx context.Context, bConfig BConfig) (*AppConfig, error) {
	cli, err := etcdLib.New(etcdLib.Config{
		Endpoints:   strings.Split(bConfig.EtcdAddrs, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, lib.NewXError(err, "Connect Etcd Failed")
	}
	defer cli.Close()

	bKeyPath := []byte(bConfig.AppConfPath())
	gResp, err := cli.Get(ctx, bConfig.AppConfPath())
	if err != nil {
		return nil, lib.NewXError(err, fmt.Sprintf("Get %v Config Data Fail", bConfig.LoadAppId()))
	}
	for _, kv := range gResp.Kvs {
		key := kv.Key
		if !reflect.DeepEqual(key, bKeyPath) {
			continue
		}

		value := kv.Value

		var _appConfig AppConfig
		if xerr := ReadFromJSON(value, &_appConfig); xerr != nil {
			logx.Errorf("%v App配置有误: %v", bConfig.LoadAppId(), xerr)
			continue
		}
		return &_appConfig, nil
	}
	return nil, lib.NewXError(fmt.Errorf("%v App配置未找到", bConfig.LoadAppId()), "")
}
