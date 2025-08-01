package config

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ruandao/distribute-im-pkg/lib"
	"github.com/spf13/viper"
)

type BConfig struct {
	BusinessName     string   `mapstructure:"business_name"`
	Version          string   `mapstructure:"version"`
	IP               string   `mapstructure:"ip"`
	Port             string   `mapstructure:"port"`
	EtcdAddrs string 	`mapstructure:"etcd_addrs"`
	Lease            int64    `mapstructure:"lease_time_seconds"`
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
	if !fileExists("must.env") {
		return config, lib.NewXError(fmt.Errorf("must.env missed"), "")
	}
	fmt.Println("must.env exist")

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

	// 获取所有环境变量键名
	keys := viper.AllKeys()
	// 排序键名以便更好的展示
	sort.Strings(keys)
	fmt.Println("所有环境变量:")
	fmt.Println("-------------------")

	// 遍历并打印所有环境变量
	for _, key := range keys {
		value := viper.GetString(key)
		// 为了安全，这里可以过滤掉敏感信息，如包含"PASSWORD"、"TOKEN"等关键词的变量
		if isSensitive(key) {
			fmt.Printf("%s: ******\n", key)
		} else {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("===================")
	
	fd, err := os.OpenFile("must.env", os.O_RDONLY, os.ModeAppend); 
	if err !=nil {
		return  config, lib.NewXError(err, "must.env missed")
	}
	content, err := io.ReadAll(fd)
	if err != nil {
		return  config, lib.NewXError(err, "must.env read err")
	}
	lines := strings.Split(string(content), "\n")
	problemEnvs := []string{}
	for _, line := range lines {
		key := strings.Trim(line, "")
		if key != "" {
			val := viper.GetString(key)
			if val == "" {
				problemEnvs = append(problemEnvs, key)
			}
		}
	}
	if len(problemEnvs) >= 1 {
		fmt.Printf("be short of these environments: %v\n", problemEnvs)
		os.Exit(1)
	}


	if err := viper.Unmarshal(&config); err != nil {
		return config, lib.NewXError(err, "config.yaml parse fail....")
	}

	return config, nil
}

// 文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true // 文件存在
	}
	if os.IsNotExist(err) {
		return false // 文件不存在
	}
	// 其他错误（如权限问题等）
	return false
}

// 判断是否为敏感环境变量
func isSensitive(key string) bool {
	sensitiveKeywords := []string{"PASSWORD", "TOKEN", "SECRET", "KEY", "CREDENTIAL"}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(strings.ToUpper(key), kw) {
			return true
		}
	}
	return false
}
