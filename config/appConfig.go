package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	sort "github.com/ruandao/distribute-im-pkg/config/util/sortDepList"

	lib "github.com/ruandao/distribute-im-pkg/lib"
	"github.com/ruandao/distribute-im-pkg/xetcd"
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

func RegisteredDependentServiceList(depList []string) {
	// depList 是硬编码在代码中的, 当前服务依赖基础服务
	// depListVal, 是一个临时容器, 不直接使用变量是为了避免cpu的缓存问题
	// depListVal, 会在后面跟 etcd 中读取的服务依赖做比较,确保 进行部署的同学知道服务的依赖情况
	depListVal.Store(depList)
	fmt.Printf("current service dependent on these services: %v\n", depList)

	once.Do(func() {
		depList := depListVal.Load().([]string)
		sort.SortInplace(depList)

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

	etcdContent, err := xetcd.New(ctx, strings.Split(bConfig.EtcdAddrs, ","))
	if err != nil {
		return nil, lib.NewXError(err, "Connect Etcd Failed")
	}
	defer etcdContent.Close()
	configPath, appConfigStr, found := etcdContent.GetSelfConfig(bConfig.BusinessName, bConfig.Role, bConfig.Version, bConfig.ShareName)
	if !found {
		return nil, lib.NewXError(fmt.Errorf("app %v config not found on etcd for path: %v", bConfig.LoadAppId(), configPath), "")
	}
	fmt.Printf("appConfig: %v\n", appConfigStr)

	var _appConfig AppConfig
	if xerr := lib.ReadFromJSON([]byte(appConfigStr), &_appConfig); xerr != nil {
		return nil, lib.NewXError(xerr, fmt.Sprintf("%v App配置有误", bConfig.LoadAppId()))
	}
	
	return &_appConfig, nil
}
