package config

import (
	"fmt"

	"github.com/ruandao/distribute-im-pkg/lib"
	"github.com/spf13/viper"
)

type BConfig struct {
	BusinessName     string   `mapstructure:"business-name"`
	Version          string   `mapstructure:"version"`
	IP               string   `mapstructure:"ip"`
	Port             string   `mapstructure:"port"`
	EtcdConfigCenter []string `mapstructure:"etcd-config-center"`
	Lease            int64    `mapstructure:"lease-time-seconds"`
}

func (bConfig BConfig) AppConfPath() string {
	keyPath := fmt.Sprintf("/service/%v/%v/config", bConfig.BusinessName, bConfig.Version)
	return keyPath
}
func (bConf BConfig) ListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%v", bConf.Port)
}
func (bConf BConfig) RegisterAddr() string {
	return fmt.Sprintf("%v:%v", bConf.IP, bConf.Port)
}

func (bConf BConfig) LoadAppId() string {
	return fmt.Sprintf("%v-%v", bConf.BusinessName, bConf.Version)
}

func LoadBasicConfig() (BConfig, error) {
	config := BConfig{}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return config, lib.NewXError(err, "load config.yaml fail")
	}

	// 优先使用环境变量（需设置环境变量前缀）
	viper.AutomaticEnv()
	viper.SetEnvPrefix("APP")     // 环境变量需以APP_开头，如APP_DATABASE_URL
	viper.BindEnv("port", "PORT") // APP_PORT=8901 go run .  ， 这样就把端口修改为8901

	fmt.Printf("All config keys: %v\n", viper.AllKeys())
	if err := viper.Unmarshal(&config); err != nil {
		return config, lib.NewXError(err, "config.yaml parse fail....")
	}

	return config, nil
}
