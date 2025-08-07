package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"

	confreadych "github.com/ruandao/distribute-im-pkg/config/confReadyCh"
	sortdeplist "github.com/ruandao/distribute-im-pkg/config/util/sortDepList"

	lib "github.com/ruandao/distribute-im-pkg/lib"
)

type Config struct {
	BConfig
	AppConfig
}

func (conf *Config) AppStatePaths() []string {
	var paths []string
	for _, tag := range conf.TrafficTags {
		// /service/${BusinessName}/ready/${tag}/${ip:port}
		keyPath1 := fmt.Sprintf("/AppState/service/%v/%v/%v/%v", conf.BusinessName, conf.Version, tag, conf.RegisterAddr())
		// the client don't know which is the version of target business
		// it only know the businessName and TrafficTag
		keyPath2 := fmt.Sprintf("/AppState/traffic/%v/%v/%v", tag, conf.BusinessName, conf.RegisterAddr())
		paths = append(paths, keyPath1, keyPath2)
	}
	return paths
}

func (conf *Config) AppTrafficPrefix(tag string) string {
	return fmt.Sprintf("/AppState/traffic/%v", tag)
}

// func (conf *Config) GetTrafficRouteForBusiness(trafficTag string, businessName string) {
// 	for _, tag := range conf.TrafficTags {
// 		if tag == trafficTag {

// 		}
// 	}
// }

var configVal atomic.Value

func readConf() *Config {
	if conf := configVal.Load(); conf != nil {
		return conf.(*Config)
	}
	return nil
}

func writeAppConf(appConf AppConfig) error {
	sortdeplist.SortInplace(appConf.DepServices)
	depList := depListVal.Load().([]string)
	if !reflect.DeepEqual(appConf.DepServices, depList) {
		err := errors.New(
			"AppConf.DepServices error: \n" +
				fmt.Sprintf("The list should be %v \n", depList) +
				fmt.Sprintf("But get %v", appConf.DepServices),
		)
		return lib.NewXError(err, "")
	}

	conf := readConf()
	if conf == nil {
		conf = &Config{}
	}
	conf.AppConfig = appConf
	configVal.Store(conf)
	return nil
}
func writeBConf(bConf BConfig) {
	conf := readConf()
	if conf == nil {
		conf = &Config{}
	}
	conf.BConfig = bConf
	configVal.Store(conf)
}

func LoadConfig(ctx context.Context, depList []string) (*Config, error) {
	RegisteredDependentServiceList(depList)

	bConfig, xerr := LoadBasicConfig()
	if xerr != nil {
		return nil, xerr
	}
	writeBConf(bConfig)
	fmt.Printf("bConfig: %v\n", bConfig)
	fmt.Printf("ListenAddr: %v\n", bConfig.ListenAddr())
	fmt.Printf("RegisterAddr: %v\n", bConfig.RegisterAddr())

	_appConf, xerr := readAppConfig(ctx, bConfig)
	if xerr != nil {
		return nil, xerr
	}

	if !sortdeplist.DepListEq(depList, _appConf.DepServices) {
		return nil, lib.NewXError(fmt.Errorf("应用需要的依赖服务和配置中心注册的依赖服务不一致"), fmt.Sprintf(" appDepList: %v\n etcdDepList: %v\n", depList, _appConf.DepServices))
	}

	writeAppConf(*_appConf)
	confreadych.Close()

	return readConf(), xerr
}
